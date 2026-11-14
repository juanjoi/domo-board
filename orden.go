package main

import (
	"slices"
	"strings"
)

// redesCompose lee la sección networks de primer nivel de un compose y
// devuelve las redes que crea y las que usa como externas, por su nombre en
// Docker (name, o la clave si no tiene). No es un lector de YAML completo:
// basta con el formato de bloque habitual de los compose.
func redesCompose(compose string) (crea, usa []string) {
	var (
		dentro           bool
		clave, nombre    string
		externa, abierta bool
		sangriaEntrada   = -1
	)
	cerrar := func() {
		if !abierta {
			return
		}
		if nombre == "" {
			nombre = clave
		}
		if externa {
			usa = append(usa, nombre)
		} else {
			crea = append(crea, nombre)
		}
		clave, nombre, externa, abierta = "", "", false, false
	}
	for _, linea := range strings.Split(compose, "\n") {
		sinComentario, _, _ := strings.Cut(linea, "#")
		texto := strings.TrimRight(sinComentario, " \t\r")
		if strings.TrimSpace(texto) == "" {
			continue
		}
		sangria := len(texto) - len(strings.TrimLeft(texto, " "))
		if sangria == 0 {
			cerrar()
			dentro = texto == "networks:"
			sangriaEntrada = -1
			continue
		}
		if !dentro {
			continue
		}
		if sangriaEntrada < 0 {
			sangriaEntrada = sangria
		}
		campo, valor, _ := strings.Cut(strings.TrimSpace(texto), ":")
		valor = strings.Trim(strings.TrimSpace(valor), `"'`)
		switch {
		case sangria == sangriaEntrada:
			cerrar()
			clave, abierta = campo, true
		case sangria > sangriaEntrada && campo == "name":
			nombre = valor
		case sangria > sangriaEntrada && campo == "external":
			externa = valor == "true"
		}
	}
	cerrar()
	return crea, usa
}

// ordenarStacks pone primero los stacks que crean redes que otros del mismo
// lote usan como externas. Entre los que no dependen unos de otros mantiene el
// orden alfabético. Si hay un ciclo, el resto va en orden alfabético.
func ordenarStacks(stacks []string, composes map[string]string) []string {
	pendientes := slices.Clone(stacks)
	slices.Sort(pendientes)
	creador := map[string]string{}
	usa := map[string][]string{}
	for _, s := range pendientes {
		c, u := redesCompose(composes[s])
		for _, red := range c {
			creador[red] = s
		}
		usa[s] = u
	}
	// depende[s] son los stacks del lote que tienen que ir antes que s.
	depende := map[string][]string{}
	for _, s := range pendientes {
		for _, red := range usa[s] {
			if c, ok := creador[red]; ok && c != s && !slices.Contains(depende[s], c) {
				depende[s] = append(depende[s], c)
			}
		}
	}
	var orden []string
	for len(pendientes) > 0 {
		i := slices.IndexFunc(pendientes, func(s string) bool {
			for _, d := range depende[s] {
				if !slices.Contains(orden, d) {
					return false
				}
			}
			return true
		})
		if i < 0 {
			return append(orden, pendientes...)
		}
		orden = append(orden, pendientes[i])
		pendientes = slices.Delete(pendientes, i, i+1)
	}
	return orden
}
