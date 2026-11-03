package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// entorno crea un repositorio "remoto" y un clon, como el del servidor.
type entorno struct {
	t       *testing.T
	trabajo string // donde se hacen los commits
	clon    string
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "prueba")
	t.Setenv("GIT_AUTHOR_EMAIL", "prueba@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "prueba")
	t.Setenv("GIT_COMMITTER_EMAIL", "prueba@example.com")
	base := t.TempDir()
	e := &entorno{t: t, trabajo: filepath.Join(base, "trabajo"), clon: filepath.Join(base, "clon")}
	e.git(base, "init", "-q", "--bare", "-b", "main", "remoto.git")
	e.git(base, "clone", "-q", "remoto.git", "trabajo")
	return e
}

func (e *entorno) git(dir string, args ...string) string {
	e.t.Helper()
	salida, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		e.t.Fatalf("git %v: %v\n%s", args, err, salida)
	}
	return string(salida)
}

// commit escribe los ficheros (vacío = borrar) y sube el commit.
func (e *entorno) commit(mensaje string, ficheros map[string]string) string {
	e.t.Helper()
	for ruta, contenido := range ficheros {
		completa := filepath.Join(e.trabajo, ruta)
		if contenido == "" {
			os.Remove(completa)
			continue
		}
		os.MkdirAll(filepath.Dir(completa), 0o755)
		if err := os.WriteFile(completa, []byte(contenido), 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
	e.git(e.trabajo, "add", "-A")
	e.git(e.trabajo, "commit", "-q", "-m", mensaje)
	e.git(e.trabajo, "push", "-q", "origin", "main")
	return e.git(e.trabajo, "rev-parse", "HEAD")[:40]
}

func (e *entorno) clonar() *Repo {
	e.git(filepath.Dir(e.clon), "clone", "-q", "remoto.git", "clon")
	return &Repo{Dir: e.clon, Rama: "main"}
}

func leer(t *testing.T, ruta string) string {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return ""
	}
	return string(datos)
}

func TestStacksYCambios(t *testing.T) {
	ctx := context.Background()
	e := nuevoEntorno(t)
	c1 := e.commit("inicial", map[string]string{
		"stacks/alfa/docker-compose.yml": "v1",
		"stacks/beta/docker-compose.yml": "v1",
		"stacks/sin-compose/notas.md":    "x",
		"docs/leeme.md":                  "x",
	})
	repo := e.clonar()
	c2 := e.commit("cambia alfa", map[string]string{"stacks/alfa/docker-compose.yml": "v2"})

	if err := repo.Buscar(ctx); err != nil {
		t.Fatal(err)
	}
	ultimo, err := repo.Ultimo(ctx)
	if err != nil || ultimo != c2 {
		t.Fatalf("Ultimo = %q, %v; quería %q", ultimo, err, c2)
	}
	stacks, _ := repo.Stacks(ctx, ultimo)
	if !slices.Equal(stacks, []string{"alfa", "beta"}) {
		t.Errorf("Stacks = %v", stacks)
	}
	if cambios, _ := repo.Cambios(ctx, c1, c2, "alfa"); len(cambios) != 1 || cambios[0].Mensaje != "cambia alfa" {
		t.Errorf("Cambios alfa = %v", cambios)
	}
	if cambios, _ := repo.Cambios(ctx, c1, c2, "beta"); len(cambios) != 0 {
		t.Errorf("Cambios beta = %v", cambios)
	}
	// Buscar no toca los ficheros: alfa sigue como estaba.
	if got := leer(t, filepath.Join(e.clon, "stacks/alfa/docker-compose.yml")); got != "v1" {
		t.Errorf("tras buscar, alfa = %q", got)
	}
}

func TestRestaurarSoloElStack(t *testing.T) {
	ctx := context.Background()
	e := nuevoEntorno(t)
	e.commit("inicial", map[string]string{
		"stacks/alfa/docker-compose.yml": "v1",
		"stacks/alfa/viejo.yml":          "x",
		"stacks/beta/docker-compose.yml": "v1",
	})
	repo := e.clonar()
	c2 := e.commit("cambia todo", map[string]string{
		"stacks/alfa/docker-compose.yml":  "v2",
		"stacks/alfa/viejo.yml":           "",
		"stacks/alfa/nuevo.yml":           "y",
		"stacks/beta/docker-compose.yml":  "v2",
		"stacks/gamma/docker-compose.yml": "v1",
	})
	repo.Buscar(ctx)

	if err := repo.Restaurar(ctx, c2, "alfa"); err != nil {
		t.Fatal(err)
	}
	if got := leer(t, filepath.Join(e.clon, "stacks/alfa/docker-compose.yml")); got != "v2" {
		t.Errorf("alfa = %q", got)
	}
	if _, err := os.Stat(filepath.Join(e.clon, "stacks/alfa/viejo.yml")); !os.IsNotExist(err) {
		t.Errorf("viejo.yml debería haberse borrado")
	}
	if got := leer(t, filepath.Join(e.clon, "stacks/alfa/nuevo.yml")); got != "y" {
		t.Errorf("nuevo.yml = %q", got)
	}
	if got := leer(t, filepath.Join(e.clon, "stacks/beta/docker-compose.yml")); got != "v1" {
		t.Errorf("beta no debería cambiar, = %q", got)
	}
	// Un stack nuevo, que aún no está en el clon, también se restaura.
	if err := repo.Restaurar(ctx, c2, "gamma"); err != nil {
		t.Fatal(err)
	}
	if got := leer(t, filepath.Join(e.clon, "stacks/gamma/docker-compose.yml")); got != "v1" {
		t.Errorf("gamma = %q", got)
	}
}
