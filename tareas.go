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
	EnCola    = "en cola"
	EnCurso   = "en curso"
	Correcta  = "correcta"
	Fallida   = "fallida"
	Cancelada = "cancelada"
)

var idValido = regexp.MustCompile(`^\d{8}T\d{6}Z-[a-z0-9][a-z0-9_-]*$`)

// Tarea es una actualización de un stack. Cada una se guarda en su propio JSON,
// junto a su log, para que el panel y el contenedor de la tarea no escriban en
// el mismo fichero. Las tareas que se piden juntas forman un lote: se aplican
// en orden y, si una falla, se cancelan las siguientes del lote.
type Tarea struct {
	ID     string    `json:"id"`
	Stack  string    `json:"stack"`
	Commit string    `json:"commit"`
	Estado string    `json:"estado"`
	Error  string    `json:"error,omitempty"`
	Lote   string    `json:"lote,omitempty"`
	Orden  int64     `json:"orden,omitempty"` // posición en la cola
	Inicio time.Time `json:"inicio"`          // cuándo se pidió
	Fin    time.Time `json:"fin"`
}

func nuevaTarea(stack, commit string, ahora time.Time) Tarea {
	return Tarea{
		ID:     ahora.UTC().Format("20060102T150405Z") + "-" + stack,
		Stack:  stack,
		Commit: commit,
		Estado: EnCola,
		Orden:  ahora.UnixNano(),
		Inicio: ahora,
	}
}

// nuevoLote crea las tareas de varios stacks, ya ordenados, en un mismo lote.
func nuevoLote(stacks []string, commit string, ahora time.Time) []Tarea {
	var tareas []Tarea
	for i, stack := range stacks {
		t := nuevaTarea(stack, commit, ahora)
		t.Lote = t.ID
		if i > 0 {
			t.Lote = tareas[0].Lote
		}
		t.Orden += int64(i)
		tareas = append(tareas, t)
	}
	return tareas
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

// Existe dice si ya hay una tarea con ese id.
func (r Registro) Existe(id string) bool {
	_, err := os.Stat(r.ruta(id))
	return err == nil
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
	sort.Slice(tareas, func(i, j int) bool {
		if !tareas[i].Inicio.Equal(tareas[j].Inicio) {
			return tareas[i].Inicio.After(tareas[j].Inicio)
		}
		return tareas[i].Orden > tareas[j].Orden
	})
	return tareas, nil
}

// pendientes devuelve las tareas en cola, en el orden en que se aplican.
func pendientes(tareas []Tarea) []Tarea {
	var cola []Tarea
	for _, t := range tareas {
		if t.Estado == EnCola {
			cola = append(cola, t)
		}
	}
	sort.Slice(cola, func(i, j int) bool { return cola[i].Orden < cola[j].Orden })
	return cola
}

// activos devuelve los stacks con una tarea en cola o en curso.
func activos(tareas []Tarea) map[string]string {
	resultado := map[string]string{}
	for _, t := range tareas {
		if t.Estado == EnCola || t.Estado == EnCurso {
			if resultado[t.Stack] != EnCurso {
				resultado[t.Stack] = t.Estado
			}
		}
	}
	return resultado
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
