package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	nombreValido = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	commitValido = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// Repo es el clon del repositorio de despliegue. El panel nunca mueve HEAD:
// descarga con fetch y, al actualizar un stack, restaura solo su carpeta.
// Así un cambio en otro stack no entra en vigor hasta que se aplica.
type Repo struct {
	Dir  string
	Rama string
	mu   sync.Mutex
}

// Cambio es un commit que toca un stack.
type Cambio struct {
	Commit  string
	Mensaje string
}

func (r *Repo) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.Dir}, args...)...)
	var salida, errores bytes.Buffer
	cmd.Stdout = &salida
	cmd.Stderr = &errores
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errores.String()))
	}
	return strings.TrimSpace(salida.String()), nil
}

func (r *Repo) remota() string { return "origin/" + r.Rama }

// Buscar descarga los commits nuevos de la rama sin tocar los ficheros.
func (r *Repo) Buscar(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := r.git(ctx, "fetch", "--prune", "origin", r.Rama)
	return err
}

// Ultimo devuelve el último commit descargado de la rama.
func (r *Repo) Ultimo(ctx context.Context) (string, error) {
	return r.git(ctx, "rev-parse", "--verify", r.remota()+"^{commit}")
}

// Stacks devuelve las carpetas de stacks/ que tienen docker-compose.yml en un commit.
func (r *Repo) Stacks(ctx context.Context, commit string) ([]string, error) {
	salida, err := r.git(ctx, "ls-tree", "-r", "--name-only", commit, "--", "stacks/")
	if err != nil {
		return nil, err
	}
	var stacks []string
	for _, ruta := range strings.Split(salida, "\n") {
		partes := strings.Split(ruta, "/")
		if len(partes) == 3 && partes[2] == "docker-compose.yml" && nombreValido.MatchString(partes[1]) {
			stacks = append(stacks, partes[1])
		}
	}
	sort.Strings(stacks)
	return stacks, nil
}

// Compose devuelve el docker-compose.yml de un stack en un commit.
func (r *Repo) Compose(ctx context.Context, commit, stack string) (string, error) {
	return r.git(ctx, "show", commit+":stacks/"+stack+"/docker-compose.yml")
}

// Cambios devuelve los commits entre desde y hasta que tocan un stack.
func (r *Repo) Cambios(ctx context.Context, desde, hasta, stack string) ([]Cambio, error) {
	salida, err := r.git(ctx, "log", "--format=%h%x09%s", desde+".."+hasta, "--", "stacks/"+stack+"/")
	if err != nil || salida == "" {
		return nil, err
	}
	var cambios []Cambio
	for _, linea := range strings.Split(salida, "\n") {
		commit, mensaje, _ := strings.Cut(linea, "\t")
		cambios = append(cambios, Cambio{Commit: commit, Mensaje: mensaje})
	}
	return cambios, nil
}

// Restaurar deja la carpeta de un stack como está en un commit, incluidos los
// ficheros que se borraron en él.
func (r *Repo) Restaurar(ctx context.Context, commit, stack string) error {
	_, err := r.git(ctx, "restore", "--source="+commit, "--worktree", "--", "stacks/"+stack+"/")
	return err
}

func corto(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
