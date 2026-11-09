package main

import (
	"context"
	"embed"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

//go:embed plantillas/*.html
var plantillasFS embed.FS

const (
	intervaloBusqueda = 15 * time.Minute
	maxLog            = 256 << 10 // se muestra el final del log
	numTareas         = 10
)

type Servidor struct {
	cfg        Config
	repo       *Repo
	reg        Registro
	docker     *Docker
	plantillas *template.Template
	ahora      func() time.Time

	mu             sync.Mutex
	ultimaBusqueda time.Time
	errorBusqueda  string
}

func nuevoServidor(cfg Config, docker *Docker) *Servidor {
	return &Servidor{
		cfg:    cfg,
		repo:   &Repo{Dir: cfg.Repo, Rama: cfg.Rama},
		reg:    Registro{Dir: filepath.Join(cfg.Datos, "tareas")},
		docker: docker,
		plantillas: template.Must(template.New("").Funcs(template.FuncMap{
			"corto": corto,
			"fecha": func(t time.Time) string { return t.Local().Format("02/01/2006 15:04") },
		}).ParseFS(plantillasFS, "plantillas/*.html")),
		ahora: time.Now,
	}
}

func servir(ctx context.Context, cfg Config) error {
	s := nuevoServidor(cfg, nuevoDocker())
	go s.buscarPeriodicamente(ctx)
	srv := &http.Server{Addr: cfg.Direccion, Handler: s.rutas(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	log.Printf("domo-board escuchando en %s", cfg.Direccion)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Servidor) rutas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.inicio)
	mux.HandleFunc("GET /tareas/{id}", s.verTarea)
	mux.HandleFunc("POST /buscar", s.buscar)
	mux.HandleFunc("POST /actualizar", s.actualizar)
	mux.HandleFunc("GET /salud", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })
	return mismoOrigen(mux)
}

// mismoOrigen rechaza los formularios enviados desde otra web: el panel no
// tiene login, así que cualquier página abierta en el navegador podría
// intentar pulsar sus botones.
func mismoOrigen(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			sitio := r.Header.Get("Sec-Fetch-Site")
			origen, _ := url.Parse(r.Header.Get("Origin"))
			if (sitio != "" && sitio != "same-origin" && sitio != "none") ||
				(origen != nil && origen.Host != "" && origen.Host != r.Host) {
				http.Error(w, "Petición de otro origen", http.StatusForbidden)
				return
			}
		}
		siguiente.ServeHTTP(w, r)
	})
}

func (s *Servidor) buscarPeriodicamente(ctx context.Context) {
	for {
		s.buscarCambios(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(intervaloBusqueda):
		}
	}
}

func (s *Servidor) buscarCambios(ctx context.Context) {
	err := s.repo.Buscar(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ultimaBusqueda = s.ahora()
	s.errorBusqueda = ""
	if err != nil {
		s.errorBusqueda = err.Error()
		log.Print(err)
	}
}

type FilaStack struct {
	Nombre      string
	Enlaces     []Enlace // servicios que publica a través de Traefik
	Desplegado  string   // commit de la última tarea correcta, vacío si no hay
	Cambios     []Cambio
	Desconocido bool // no se sabe qué hay desplegado
	UltimaTarea *Tarea
}

type VistaInicio struct {
	Ultimo         string
	Stacks         []FilaStack
	Huerfanos      []string
	EnCurso        bool
	Tareas         []Tarea
	UltimaBusqueda time.Time
	Errores        []string
}

func (s *Servidor) inicio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	v := VistaInicio{}
	s.mu.Lock()
	v.UltimaBusqueda = s.ultimaBusqueda
	if s.errorBusqueda != "" {
		v.Errores = append(v.Errores, "No se pudieron buscar cambios: "+s.errorBusqueda)
	}
	s.mu.Unlock()

	enCurso, err := s.docker.TareaEnCurso(ctx)
	v.EnCurso = enCurso
	if err != nil {
		v.Errores = append(v.Errores, err.Error())
	}
	tareas, err := s.reg.Todas()
	if err != nil {
		v.Errores = append(v.Errores, err.Error())
	}
	v.Tareas = tareas[:min(len(tareas), numTareas)]

	v.Ultimo, err = s.repo.Ultimo(ctx)
	if err != nil {
		v.Errores = append(v.Errores, err.Error())
		s.pintar(w, "inicio.html", v)
		return
	}
	stacks, err := s.repo.Stacks(ctx, v.Ultimo)
	if err != nil {
		v.Errores = append(v.Errores, err.Error())
	}

	ultimas := map[string]Tarea{}
	for _, t := range tareas {
		if _, visto := ultimas[t.Stack]; !visto {
			ultimas[t.Stack] = t
		}
	}
	desp := desplegados(tareas)
	for _, nombre := range stacks {
		fila := FilaStack{Nombre: nombre, Desconocido: true}
		if compose, err := s.repo.Compose(ctx, v.Ultimo, nombre); err == nil {
			fila.Enlaces = enlaces(nombresTraefik(compose), r)
		}
		if t, ok := ultimas[nombre]; ok {
			fila.UltimaTarea = &t
		}
		if t, ok := desp[nombre]; ok {
			fila.Desplegado = t.Commit
			cambios, err := s.repo.Cambios(ctx, t.Commit, v.Ultimo, nombre)
			fila.Cambios, fila.Desconocido = cambios, err != nil
		}
		v.Stacks = append(v.Stacks, fila)
	}
	for nombre := range desp {
		if !slices.Contains(stacks, nombre) {
			v.Huerfanos = append(v.Huerfanos, nombre)
		}
	}
	slices.Sort(v.Huerfanos)
	s.pintar(w, "inicio.html", v)
}

func (s *Servidor) buscar(w http.ResponseWriter, r *http.Request) {
	s.buscarCambios(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// actualizar solo acepta un stack que exista en el commit que se vio en la
// página: el panel controla todo el servidor y no admite parámetros libres.
func (s *Servidor) actualizar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stack, commit := r.FormValue("stack"), r.FormValue("commit")
	if !nombreValido.MatchString(stack) || !commitValido.MatchString(commit) {
		http.Error(w, "Stack o commit no válido", http.StatusBadRequest)
		return
	}
	stacks, err := s.repo.Stacks(ctx, commit)
	if err != nil || !slices.Contains(stacks, stack) {
		http.Error(w, "El stack no existe en ese commit", http.StatusBadRequest)
		return
	}
	if enCurso, err := s.docker.TareaEnCurso(ctx); err != nil || enCurso {
		http.Error(w, "Ya hay una tarea en curso; espera a que termine", http.StatusConflict)
		return
	}

	t := nuevaTarea(stack, commit, s.ahora())
	if err := s.reg.Guardar(t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.docker.Lanzar(ctx, s.cfg, t.ID); err != nil {
		t.Estado, t.Error, t.Fin = Fallida, "No se pudo lanzar la tarea: "+err.Error(), s.ahora()
		s.reg.Guardar(t)
	}
	http.Redirect(w, r, "/tareas/"+t.ID, http.StatusSeeOther)
}

type VistaTarea struct {
	Tarea        Tarea
	Interrumpida bool
	Log          string
}

func (s *Servidor) verTarea(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.reg.Leer(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v := VistaTarea{Tarea: t, Log: leerFinal(s.reg.RutaLog(id), maxLog)}
	if t.Estado == EnCurso {
		enCurso, err := s.docker.TareaEnCurso(r.Context())
		v.Interrumpida = err == nil && !enCurso
	}
	s.pintar(w, "tarea.html", v)
}

func leerFinal(ruta string, maximo int64) string {
	f, err := os.Open(ruta)
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > maximo {
		f.Seek(-maximo, io.SeekEnd)
	}
	datos, _ := io.ReadAll(f)
	return string(datos)
}

func (s *Servidor) pintar(w http.ResponseWriter, plantilla string, datos any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.plantillas.ExecuteTemplate(w, plantilla, datos); err != nil {
		log.Print(err)
	}
}
