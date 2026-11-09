package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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
	if len(tareas) != 1 || tareas[0].Stack != "alfa" || tareas[0].Commit != commit || tareas[0].Estado != EnCurso {
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
		{"tarea en curso", url.Values{"stack": {"alfa"}, "commit": {commit}}, nil, true, http.StatusConflict},
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

func TestInicio(t *testing.T) {
	s, _, commit := servidorDePrueba(t)
	// Un stack desplegado que ya no está en el repositorio aparece como huérfano.
	viejo := nuevaTarea("viejo", commit, s.ahora())
	viejo.Estado = Correcta
	s.reg.Guardar(viejo)

	w := httptest.NewRecorder()
	s.rutas().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	cuerpo := w.Body.String()
	for _, esperado := range []string{"alfa", "No desplegado desde el panel", "Ya no están en el repositorio", "viejo", commit} {
		if !strings.Contains(cuerpo, esperado) {
			t.Errorf("falta %q en la página", esperado)
		}
	}
}

func TestInicioEnlaces(t *testing.T) {
	e := nuevoEntorno(t)
	e.commit("inicial", map[string]string{
		"stacks/alfa/docker-compose.yml": "services:\n  alfa:\n    labels:\n      - traefik.http.routers.alfa.rule=HostRegexp(`^alfa\\.`)\n",
		"stacks/beta/docker-compose.yml": "services:\n  beta:\n    image: beta\n",
	})
	e.clonar()
	s := nuevoServidor(Config{Repo: e.clon, Datos: t.TempDir(), Rama: "main"}, (&dockerFalso{}).docker())

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "panel.casa.example"
	w := httptest.NewRecorder()
	s.rutas().ServeHTTP(w, r)
	cuerpo := w.Body.String()
	if !strings.Contains(cuerpo, `href="http://alfa.casa.example/"`) {
		t.Errorf("falta el enlace de alfa:\n%s", cuerpo)
	}
	if strings.Contains(cuerpo, "beta.casa.example") {
		t.Errorf("beta no se publica y no debería tener enlace")
	}
}
