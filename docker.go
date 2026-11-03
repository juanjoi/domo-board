package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// nombreTarea es fijo: Docker no deja crear dos contenedores con el mismo
// nombre, así que nunca corren dos tareas a la vez.
const nombreTarea = "domo-board-tarea"

// Docker usa el cliente de Docker de la imagen.
type Docker struct {
	// ejecutar se puede sustituir en las pruebas.
	ejecutar func(ctx context.Context, args ...string) (string, error)
}

func nuevoDocker() *Docker {
	return &Docker{ejecutar: func(ctx context.Context, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "docker", args...)
		var salida, errores bytes.Buffer
		cmd.Stdout = &salida
		cmd.Stderr = &errores
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("docker %s: %v: %s", args[0], err, strings.TrimSpace(errores.String()))
		}
		return strings.TrimSpace(salida.String()), nil
	}}
}

// TareaEnCurso dice si el contenedor de una tarea está en marcha.
func (d *Docker) TareaEnCurso(ctx context.Context) (bool, error) {
	salida, err := d.ejecutar(ctx, "ps", "-q", "--filter", "name=^"+nombreTarea+"$")
	return salida != "", err
}

type montaje struct {
	Type        string
	Name        string
	Source      string
	Destination string
	RW          bool
}

type contenedor struct {
	Config struct{ Image string }
	Mounts []montaje
}

// Lanzar arranca la tarea en un contenedor efímero con la imagen y los
// montajes del propio panel. Así la tarea termina aunque actualice el panel
// y este se reinicie. No necesita red: los commits ya están descargados.
func (d *Docker) Lanzar(ctx context.Context, cfg Config, id string) error {
	yo, err := os.Hostname()
	if err != nil {
		return err
	}
	salida, err := d.ejecutar(ctx, "inspect", yo)
	if err != nil {
		return fmt.Errorf("el panel no encuentra su propio contenedor (¿se fijó hostname en el compose?): %w", err)
	}
	var inspeccion []contenedor
	if err := json.Unmarshal([]byte(salida), &inspeccion); err != nil || len(inspeccion) != 1 {
		return fmt.Errorf("respuesta inesperada de docker inspect: %v", err)
	}
	_, err = d.ejecutar(ctx, argumentosTarea(inspeccion[0], cfg, id)...)
	return err
}

func argumentosTarea(c contenedor, cfg Config, id string) []string {
	args := []string{"run", "-d", "--rm", "--name", nombreTarea, "--network", "none"}
	for _, m := range c.Mounts {
		origen := m.Source
		if m.Type == "volume" {
			origen = m.Name
		}
		v := origen + ":" + m.Destination
		if !m.RW {
			v += ":ro"
		}
		args = append(args, "-v", v)
	}
	args = append(args,
		"-e", "DOMO_REPO="+cfg.Repo,
		"-e", "DOMO_DATOS="+cfg.Datos,
		"-e", "DOMO_RAMA="+cfg.Rama,
		"-e", "TZ="+os.Getenv("TZ"),
		c.Config.Image, "aplicar", id)
	return args
}
