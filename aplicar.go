package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// aplicar corre dentro del contenedor de la tarea: restaura la carpeta del
// stack al commit elegido y lo levanta con docker compose.
func aplicar(ctx context.Context, cfg Config, id string) error {
	reg := Registro{Dir: filepath.Join(cfg.Datos, "tareas")}
	t, err := reg.Leer(id)
	if err != nil {
		return err
	}
	registro, err := os.OpenFile(reg.RutaLog(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer registro.Close()

	err = pasos(ctx, cfg, t, registro)
	t.Fin = time.Now()
	t.Estado = Correcta
	if err != nil {
		t.Estado = Fallida
		t.Error = err.Error()
		fmt.Fprintf(registro, "\nError: %v\n", err)
	}
	return reg.Guardar(t)
}

func pasos(ctx context.Context, cfg Config, t Tarea, salida io.Writer) error {
	repo := &Repo{Dir: cfg.Repo, Rama: cfg.Rama}
	fmt.Fprintf(salida, "$ git restore --source=%s stacks/%s/\n", corto(t.Commit), t.Stack)
	if err := repo.Restaurar(ctx, t.Commit, t.Stack); err != nil {
		return err
	}
	compose := filepath.Join(cfg.Repo, "stacks", t.Stack, "docker-compose.yml")
	args := []string{"compose", "-f", compose, "up", "-d", "--remove-orphans", "--wait", "--wait-timeout", "300"}
	fmt.Fprintf(salida, "$ docker %v\n", args)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = salida
	cmd.Stderr = salida
	return cmd.Run()
}
