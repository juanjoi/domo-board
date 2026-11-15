package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// dockerFalso registra las órdenes en vez de ejecutarlas.
type dockerFalso struct {
	enCurso bool
	ordenes [][]string
}

func (f *dockerFalso) docker() *Docker {
	return &Docker{ejecutar: func(_ context.Context, args ...string) (string, error) {
		f.ordenes = append(f.ordenes, args)
		switch args[0] {
		case "ps":
			if f.enCurso {
				return "abc123", nil
			}
			return "", nil
		case "inspect":
			return `[{"Config":{"Image":"domo-board:prueba"},"Mounts":[]}]`, nil
		}
		return "", nil
	}}
}

func (f *dockerFalso) lanzadas() int {
	n := 0
	for _, o := range f.ordenes {
		if o[0] == "run" {
			n++
		}
	}
	return n
}

func servidorDePrueba(t *testing.T) (*Servidor, *dockerFalso, string) {
	e := nuevoEntorno(t)
	commit := e.commit("inicial", map[string]string{"stacks/alfa/docker-compose.yml": "v1"})
	e.clonar()
	falso := &dockerFalso{}
	s := nuevoServidor(Config{Repo: e.clon, Datos: t.TempDir(), Rama: "main"}, falso.docker())
	return s, falso, commit
}

func enviar(s *Servidor, ruta string, datos url.Values, cabeceras map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, ruta, strings.NewReader(datos.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range cabeceras {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.rutas().ServeHTTP(w, r)
	return w
}

func TestActualizar(t *testing.T) {
	s, falso, commit := servidorDePrueba(t)

	w := enviar(s, "/actualizar", url.Values{"stack": {"alfa"}, "commit": {commit}}, map[string]string{"Sec-Fetch-Site": "same-origin"})
	if w.Code != http.StatusSeeOther || falso.lanzadas() != 1 {
		t.Fatalf("código %d, lanzadas %d: %s", w.Code, falso.lanzadas(), w.Body)
	}
	tareas, _ := s.reg.Todas()
	if len(tareas) != 1 || tareas[0].Stack != "alfa" || tareas[0].Commit != commit || tareas[0].Estado != EnCola {
		t.Errorf("tareas = %v", tareas)
	}
	if !strings.HasSuffix(w.Header().Get("Location"), tareas[0].ID) {
		t.Errorf("redirige a %q", w.Header().Get("Location"))
	}
}

func TestActualizarRechaza(t *testing.T) {
	s, falso, commit := servidorDePrueba(t)
	casos := []struct {
		nombre    string
		datos     url.Values
		cabeceras map[string]string
		enCurso   bool
		codigo    int
	}{
		{"desde otra web", url.Values{"stack": {"alfa"}, "commit": {commit}}, map[string]string{"Sec-Fetch-Site": "cross-site"}, false, http.StatusForbidden},
		{"origen distinto", url.Values{"stack": {"alfa"}, "commit": {commit}}, map[string]string{"Origin": "http://malo.example"}, false, http.StatusForbidden},
		{"stack inexistente", url.Values{"stack": {"omega"}, "commit": {commit}}, nil, false, http.StatusBadRequest},
		{"nombre con ruta", url.Values{"stack": {"../alfa"}, "commit": {commit}}, nil, false, http.StatusBadRequest},
		{"commit no válido", url.Values{"stack": {"alfa"}, "commit": {"main"}}, nil, false, http.StatusBadRequest},
		{"sin stacks", url.Values{"commit": {commit}}, nil, false, http.StatusBadRequest},
		{"uno bueno y otro no", url.Values{"stack": {"alfa", "omega"}, "commit": {commit}}, nil, false, http.StatusBadRequest},
	}
	for _, c := range casos {
		falso.enCurso = c.enCurso
		if w := enviar(s, "/actualizar", c.datos, c.cabeceras); w.Code != c.codigo {
			t.Errorf("%s: código %d, quería %d", c.nombre, w.Code, c.codigo)
		}
	}
	if falso.lanzadas() != 0 {
		t.Errorf("no debería haberse lanzado ninguna tarea")
	}
}

func pedir(s *Servidor, ruta, host string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, ruta, nil)
	if host != "" {
		r.Host = host
	}
	w := httptest.NewRecorder()
	s.rutas().ServeHTTP(w, r)
	return w
}

func TestInicio(t *testing.T) {
	s, _, commit := servidorDePrueba(t)
	// Un stack desplegado que ya no está en el repositorio aparece como huérfano.
	viejo := nuevaTarea("viejo", commit, s.ahora())
	viejo.Estado = Correcta
	s.reg.Guardar(viejo)

	cuerpo := pedir(s, "/", "").Body.String()
	// Sin menú, todos los stacks van en un solo grupo.
	for _, esperado := range []string{"<h2>Stacks</h2>", `href="/stacks/alfa"`, "No desplegado desde el panel", "Ya no están en el repositorio", "viejo", corto(commit)} {
		if !strings.Contains(cuerpo, esperado) {
			t.Errorf("falta %q en la página", esperado)
		}
	}
	if strings.Contains(cuerpo, "Últimas tareas") {
		t.Errorf("la página principal no debería mostrar las últimas tareas")
	}
}

// menuDePrueba sirve una lista del menú como la del servicio domo-menu.
func menuDePrueba(t *testing.T, json string) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/__menu/apps.json" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, json)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestInicioConMenu(t *testing.T) {
	e := nuevoEntorno(t)
	regla := func(n string) string {
		return "services:\n  " + n + ":\n    labels:\n      - traefik.http.routers." + n + ".rule=HostRegexp(`^" + n + "\\.`)\n"
	}
	primero := e.commit("inicial", map[string]string{
		"stacks/alfa/docker-compose.yml":  regla("alfa"),
		"stacks/beta/docker-compose.yml":  regla("beta"),
		"stacks/gamma/docker-compose.yml": "services:\n  gamma:\n    image: gamma\n",
		"stacks/delta/docker-compose.yml": "services:\n  delta:\n    labels:\n      - domo-board.categoria=domótica\n",
		"stacks/eps/docker-compose.yml":   "services:\n  eps:\n    labels:\n      domo-board.categoria: Inventada\n",
	})
	ultimo := e.commit("cambia beta", map[string]string{"stacks/beta/docker-compose.yml": regla("beta") + "    image: beta:2\n"})
	e.clonar()
	menu := menuDePrueba(t, `{"categories":["Familia","Domótica"],"apps":[
		{"name":"beta","title":"Beta","category":"Domótica","icon":"/__menu/iconos/beta.svg"},
		{"name":"alfa","title":"Alfa","category":"Familia","icon":"https://otro.example/alfa.svg"},
		{"name":"externa","title":"Externa","category":"Rara"}]}`)
	s := nuevoServidor(Config{Repo: e.clon, Datos: t.TempDir(), Rama: "main", Menu: menu}, (&dockerFalso{}).docker())
	for _, st := range []string{"alfa", "beta"} {
		ta := nuevaTarea(st, primero, s.ahora())
		ta.Estado = Correcta
		s.reg.Guardar(ta)
	}
	// alfa se está actualizando.
	enCurso := nuevaTarea("alfa", ultimo, s.ahora().Add(time.Second))
	enCurso.Estado = EnCurso
	s.reg.Guardar(enCurso)

	cuerpo := pedir(s, "/", "panel.casa.example").Body.String()
	orden := []string{"<h2>Familia</h2>", "Alfa", "<h2>Domótica</h2>", "Beta", "delta", "<h2>Otras</h2>", "Externa", "<h2>Otros stacks</h2>", "eps", "gamma"}
	resto := cuerpo
	for _, esperado := range orden {
		i := strings.Index(resto, esperado)
		if i < 0 {
			t.Fatalf("falta %q, o no va en su orden:\n%s", esperado, cuerpo)
		}
		resto = resto[i:]
	}
	for _, esperado := range []string{
		`<span class="carga"></span>`,                         // alfa, actualizando
		`<span class="marca pendiente"`,                       // beta, con cambios
		`<img src="/__menu/iconos/beta.svg"`,                  // icono del menú
		`href="http://externa.casa.example/" target="_blank"`, // sin stack: abre la aplicación
		`Actualizar todos los pendientes (1)`,
	} {
		if !strings.Contains(cuerpo, esperado) {
			t.Errorf("falta %q en la página", esperado)
		}
	}
	if strings.Contains(cuerpo, "otro.example") {
		t.Errorf("no debería usar iconos de fuera del servicio del menú")
	}
}

func TestStack(t *testing.T) {
	e := nuevoEntorno(t)
	e.commit("inicial", map[string]string{
		"stacks/alfa/docker-compose.yml": "services:\n  alfa:\n    labels:\n      - traefik.http.routers.alfa.rule=HostRegexp(`^alfa\\.`)\n",
	})
	e.clonar()
	s := nuevoServidor(Config{Repo: e.clon, Datos: t.TempDir(), Rama: "main"}, (&dockerFalso{}).docker())

	cuerpo := pedir(s, "/stacks/alfa", "panel.casa.example").Body.String()
	for _, esperado := range []string{`href="http://alfa.casa.example/"`, `name="stack" value="alfa"`, "Últimas tareas", "Todavía no hay tareas"} {
		if !strings.Contains(cuerpo, esperado) {
			t.Errorf("falta %q en la página:\n%s", esperado, cuerpo)
		}
	}
	for _, ruta := range []string{"/stacks/omega", "/stacks/..%2Falfa"} {
		if w := pedir(s, ruta, ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: código %d, quería 404", ruta, w.Code)
		}
	}
}

func TestActualizarVarios(t *testing.T) {
	e := nuevoEntorno(t)
	commit := e.commit("inicial", map[string]string{
		// alfa usa la red que crea beta: beta tiene que ir antes.
		"stacks/alfa/docker-compose.yml":  "services:\n  a:\n    image: a\nnetworks:\n  r:\n    name: red\n    external: true\n",
		"stacks/beta/docker-compose.yml":  "services:\n  b:\n    image: b\nnetworks:\n  r:\n    name: red\n",
		"stacks/gamma/docker-compose.yml": "services:\n  g:\n    image: g\n",
	})
	e.clonar()
	falso := &dockerFalso{}
	s := nuevoServidor(Config{Repo: e.clon, Datos: t.TempDir(), Rama: "main"}, falso.docker())

	w := enviar(s, "/actualizar", url.Values{"stack": {"gamma", "alfa", "beta", "alfa"}, "commit": {commit}}, nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" || falso.lanzadas() != 1 {
		t.Fatalf("código %d, Location %q, lanzadas %d", w.Code, w.Header().Get("Location"), falso.lanzadas())
	}
	tareas, _ := s.reg.Todas()
	var orden []string
	for _, ta := range pendientes(tareas) {
		orden = append(orden, ta.Stack)
		if ta.Lote != pendientes(tareas)[0].Lote {
			t.Errorf("%s no está en el mismo lote", ta.Stack)
		}
	}
	if strings.Join(orden, ",") != "beta,alfa,gamma" {
		t.Errorf("orden = %v", orden)
	}

	// Con el contenedor de tareas en marcha se encola sin lanzar otro, y un
	// stack que ya está en cola no se repite.
	falso.enCurso = true
	enviar(s, "/actualizar", url.Values{"stack": {"alfa"}, "commit": {commit}}, nil)
	if falso.lanzadas() != 1 {
		t.Errorf("lanzadas = %d, no debería lanzar otro contenedor", falso.lanzadas())
	}
	tareas, _ = s.reg.Todas()
	if len(tareas) != 3 {
		t.Errorf("hay %d tareas, alfa no debería repetirse", len(tareas))
	}
}
