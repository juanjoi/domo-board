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

// aplicar corre dentro del contenedor de las tareas y vacía la cola: aplica
// las tareas de una en una, por orden, hasta que no queda ninguna. El panel
// puede añadir tareas mientras tanto. Solo hay un contenedor de tareas a la
// vez, así que nadie más cambia su estado.
func aplicar(ctx context.Context, cfg Config) error {
	return vaciarCola(ctx, Registro{Dir: filepath.Join(cfg.Datos, "tareas")}, func(t Tarea, salida io.Writer) error {
		return pasos(ctx, cfg, t, salida)
	})
}

// vaciarCola separa el recorrido de la cola de los pasos de cada tarea, para
// poder probarlo sin Docker.
func vaciarCola(ctx context.Context, reg Registro, ejecutar func(Tarea, io.Writer) error) error {
	tareas, err := reg.Todas()
	if err != nil {
		return err
	}
	// Una tarea en curso al arrancar es de un contenedor anterior que murió.
	for _, t := range tareas {
		if t.Estado == EnCurso {
			t.Estado, t.Error, t.Fin = Fallida, "Interrumpida: el contenedor de la tarea terminó antes que ella", time.Now()
			if err := reg.Guardar(t); err != nil {
				return err
			}
		}
	}
	for ctx.Err() == nil {
		tareas, err := reg.Todas()
		if err != nil {
			return err
		}
		cola := pendientes(tareas)
		if len(cola) == 0 {
			return nil
		}
		if err := aplicarTarea(reg, cola[0], ejecutar); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func aplicarTarea(reg Registro, t Tarea, ejecutar func(Tarea, io.Writer) error) error {
	t.Estado = EnCurso
	if err := reg.Guardar(t); err != nil {
		return err
	}
	registro, err := os.OpenFile(reg.RutaLog(t.ID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer registro.Close()

	err = ejecutar(t, registro)
	t.Fin = time.Now()
	t.Estado = Correcta
	if err != nil {
		t.Estado = Fallida
		t.Error = err.Error()
		fmt.Fprintf(registro, "\nError: %v\n", err)
	}
	if err := reg.Guardar(t); err != nil {
		return err
	}
	if t.Estado == Fallida && t.Lote != "" {
		return cancelarLote(reg, t)
	}
	return nil
}

// cancelarLote cancela las tareas del lote que seguían en cola tras un fallo.
func cancelarLote(reg Registro, fallida Tarea) error {
	tareas, err := reg.Todas()
	if err != nil {
		return err
	}
	for _, t := range pendientes(tareas) {
		if t.Lote == fallida.Lote {
			t.Estado, t.Error, t.Fin = Cancelada, "Cancelada: falló "+fallida.Stack+", que iba antes en el mismo lote", time.Now()
			if err := reg.Guardar(t); err != nil {
				return err
			}
		}
	}
	return nil
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
