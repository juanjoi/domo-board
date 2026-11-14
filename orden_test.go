package main

import (
	"slices"
	"testing"
)

func TestRedesCompose(t *testing.T) {
	compose := `services:
  web:
    networks:
      - proxy
      - interna
networks:
  proxy:
    external: true
  interna:
  propia:
    name: "domotica" # comentario
    ipam:
      config:
        - subnet: 10.0.0.0/24
  ajena:
    name: otra
    external: true
volumes:
  datos:
`
	crea, usa := redesCompose(compose)
	if !slices.Equal(crea, []string{"interna", "domotica"}) {
		t.Errorf("crea = %v", crea)
	}
	if !slices.Equal(usa, []string{"proxy", "otra"}) {
		t.Errorf("usa = %v", usa)
	}
	if c, u := redesCompose("services:\n  x:\n    image: x\n"); c != nil || u != nil {
		t.Errorf("sin redes: %v %v", c, u)
	}
}

func TestOrdenarStacks(t *testing.T) {
	composes := map[string]string{
		"traefik": "networks:\n  proxy:\n    name: proxy\n",
		"app":     "networks:\n  proxy:\n    external: true\n  mqtt:\n    external: true\n",
		"mqtt":    "networks:\n  mqtt:\n    name: mqtt\n",
		"zeta":    "services:\n  z:\n    image: z\n",
	}
	got := ordenarStacks([]string{"zeta", "app", "traefik", "mqtt"}, composes)
	if want := []string{"mqtt", "traefik", "app", "zeta"}; !slices.Equal(got, want) {
		t.Errorf("orden = %v, quería %v", got, want)
	}
	// Un ciclo no se pierde: va en orden alfabético.
	ciclo := map[string]string{
		"a": "networks:\n  x:\n    name: x\n  y:\n    external: true\n",
		"b": "networks:\n  y:\n    name: y\n  x:\n    external: true\n",
	}
	if got := ordenarStacks([]string{"b", "a"}, ciclo); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("ciclo = %v", got)
	}
}
