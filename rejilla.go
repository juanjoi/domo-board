package main

import (
	"net/http"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const grupoSinApp = "Otros stacks"

// Mosaico es una aplicación en la rejilla de la página principal: su icono,
// su nombre y el estado del stack que la despliega, si está en el repositorio.
type Mosaico struct {
	Titulo  string
	Icono   string // ruta del icono en el servicio del menú; vacía, la inicial
	Inicial string
	Href    string // detalle del stack o, si no tiene, la propia aplicación
	Externo bool
	Stack   string
	Activo  string // en cola o en curso
	Marca   string // pendiente, fallo o desconocido
	Aviso   string // la marca en palabras
	Grande  bool
}

// Grupo es una categoría de la rejilla.
type Grupo struct {
	Nombre   string
	Mosaicos []Mosaico
}

// agrupar reparte las aplicaciones del menú en sus categorías, en el orden del
// menú y con «Otras» al final. Cada aplicación se une al stack que publica su
// nombre en Traefik. Los stacks sin aplicación en el menú van en un último
// grupo; sin menú, todos los stacks van en uno solo.
func agrupar(lista *ListaMenu, filas []FilaStack, r *http.Request) []Grupo {
	porNombre := map[string]int{}
	for i, f := range filas {
		for _, n := range f.Nombres {
			if _, visto := porNombre[n]; !visto {
				porNombre[n] = i
			}
		}
	}
	usados := map[string]bool{}
	var grupos []Grupo

	if lista != nil {
		orden := append(slices.Clone(lista.Categorias), categoriaOtras)
		porCategoria := map[string][]Mosaico{}
		for _, app := range lista.Apps {
			nombre := app.Host
			if nombre == "" {
				nombre = app.Nombre + "."
			}
			titulo := app.Titulo
			if titulo == "" {
				titulo = app.Nombre
			}
			var m Mosaico
			if i, ok := porNombre[nombre]; ok {
				m = mosaicoDe(filas[i], titulo, app.Icono)
				usados[filas[i].Nombre] = true
			} else {
				m = Mosaico{Titulo: titulo, Icono: app.Icono, Inicial: inicial(titulo)}
				if e := enlaces([]string{nombre}, r); len(e) > 0 {
					m.Href, m.Externo = e[0].URL, true
				}
			}
			categoria := app.Categoria
			if !slices.Contains(lista.Categorias, categoria) {
				categoria = categoriaOtras
			}
			porCategoria[categoria] = append(porCategoria[categoria], m)
		}
		for _, c := range orden {
			if len(porCategoria[c]) > 0 {
				grupos = append(grupos, Grupo{Nombre: c, Mosaicos: porCategoria[c]})
			}
		}
	}

	resto := Grupo{Nombre: grupoSinApp}
	if lista == nil {
		resto.Nombre = "Stacks"
	}
	for _, f := range filas {
		if !usados[f.Nombre] {
			resto.Mosaicos = append(resto.Mosaicos, mosaicoDe(f, f.Nombre, ""))
		}
	}
	if len(resto.Mosaicos) > 0 {
		grupos = append(grupos, resto)
	}
	return grupos
}

func mosaicoDe(f FilaStack, titulo, icono string) Mosaico {
	m := Mosaico{
		Titulo:  titulo,
		Icono:   icono,
		Inicial: inicial(titulo),
		Href:    "/stacks/" + f.Nombre,
		Stack:   f.Nombre,
		Activo:  f.Activo,
	}
	switch {
	case f.UltimaTarea != nil && f.UltimaTarea.Estado == Fallida:
		m.Marca, m.Aviso = "fallo", "Falló la última actualización"
	case len(f.Cambios) > 0:
		m.Marca, m.Aviso = "pendiente", "Pendiente de actualizar"
	case f.Desconocido:
		m.Marca, m.Aviso = "desconocido", "No desplegado desde el panel"
	}
	switch f.Activo {
	case EnCurso:
		m.Aviso = "Actualizando"
	case EnCola:
		m.Aviso = "En cola"
	}
	return m
}

func inicial(s string) string {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(s))
	if r == utf8.RuneError {
		return "?"
	}
	return string(unicode.ToUpper(r))
}
