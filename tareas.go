package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	EnCurso  = "en curso"
	Correcta = "correcta"
	Fallida  = "fallida"
)

var idValido = regexp.MustCompile(`^\d{8}T\d{6}Z-[a-z0-9][a-z0-9_-]*$`)

// Tarea es una actualización de un stack. Cada una se guarda en su propio JSON,
// junto a su log, para que el panel y el contenedor de la tarea no escriban en
// el mismo fichero.
type Tarea struct {
	ID     string    `json:"id"`
	Stack  string    `json:"stack"`
	Commit string    `json:"commit"`
	Estado string    `json:"estado"`
	Error  string    `json:"error,omitempty"`
	Inicio time.Time `json:"inicio"`
	Fin    time.Time `json:"fin"`
}

func nuevaTarea(stack, commit string, ahora time.Time) Tarea {
	return Tarea{
		ID:     ahora.UTC().Format("20060102T150405Z") + "-" + stack,
		Stack:  stack,
		Commit: commit,
		Estado: EnCurso,
		Inicio: ahora,
	}
}

// Registro guarda las tareas en una carpeta.
type Registro struct {
	Dir string
}

func (r Registro) ruta(id string) string { return filepath.Join(r.Dir, id+".json") }

// RutaLog es el fichero con la salida de la tarea.
func (r Registro) RutaLog(id string) string { return filepath.Join(r.Dir, id+".log") }

// Guardar escribe la tarea de forma atómica.
func (r Registro) Guardar(t Tarea) error {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return err
	}
	datos, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	temporal := r.ruta(t.ID) + ".tmp"
	if err := os.WriteFile(temporal, datos, 0o644); err != nil {
		return err
	}
	return os.Rename(temporal, r.ruta(t.ID))
}

func (r Registro) Leer(id string) (Tarea, error) {
	var t Tarea
	if !idValido.MatchString(id) {
		return t, os.ErrNotExist
	}
	datos, err := os.ReadFile(r.ruta(id))
	if err != nil {
		return t, err
	}
	return t, json.Unmarshal(datos, &t)
}

// Todas devuelve las tareas, de la más reciente a la más antigua.
func (r Registro) Todas() ([]Tarea, error) {
	entradas, err := os.ReadDir(r.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tareas []Tarea
	for _, e := range entradas {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		t, err := r.Leer(id)
		if err != nil {
			return nil, err
		}
		tareas = append(tareas, t)
	}
	sort.Slice(tareas, func(i, j int) bool { return tareas[i].Inicio.After(tareas[j].Inicio) })
	return tareas, nil
}

// desplegados devuelve, por stack, su última tarea correcta.
func desplegados(tareas []Tarea) map[string]Tarea {
	resultado := map[string]Tarea{}
	for _, t := range tareas {
		if _, visto := resultado[t.Stack]; !visto && t.Estado == Correcta {
			resultado[t.Stack] = t
		}
	}
	return resultado
}
