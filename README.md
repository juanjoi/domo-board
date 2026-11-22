# Panel del hub domótico

Aplicación web pequeña para gestionar un hub domótico basado en Docker Compose. Se despliega como un stack más y lee los stacks de un repositorio de despliegue clonado en el servidor.

## Funciones

| Función | Estado |
| --- | --- |
| Ver los stacks y si tienen cambios sin desplegar | Hecho |
| Actualizar un stack con un botón, incluido el propio panel | Hecho |
| Actualizar varios stacks a la vez, en cola y en orden | Hecho |
| Enlazar los servicios que cada stack publica en Traefik | Hecho |
| Ver las aplicaciones por categorías, con sus nombres e iconos, a partir de la lista del menú común | Hecho |
| Activar y desactivar aplicaciones | Más adelante |

## Pantalla principal

- **Rejilla de aplicaciones por categorías:** cada aplicación es su icono con el nombre debajo. Las categorías, los nombres y los iconos salen de la lista del menú común (`/__menu/apps.json`), que el panel pide al servicio del menú por la red interna y guarda 30 segundos. Si el servicio no responde, usa la última lista que consiguió.
- **Cada aplicación, con su stack:** se une al stack cuyas reglas de Traefik publican su nombre. Al pulsarla se abre la página del stack, con sus enlaces, los cambios sin desplegar, el botón *Actualizar* y sus últimas tareas. Las aplicaciones de la lista que no son de ningún stack del repositorio abren la propia aplicación.
- **Estado en el icono:** una marca en la esquina si el stack tiene cambios sin desplegar (↑), si falló su última actualización (!) o si no se ha desplegado desde el panel (?). Mientras se actualiza, una animación de carga tapa el icono; en cola, otra más lenta. La página se recarga sola mientras haya tareas en cola o en curso.
- **Dos categorías por fila**, con hasta cuatro aplicaciones por fila en cada una; en pantallas estrechas, una categoría por fila.
- **Stacks sin web:** un stack sin aplicación en el menú, como los de servicios sin web, puede ponerse en una categoría con la etiqueta `domo-board.categoria=<categoría>` en cualquier servicio de su `docker-compose.yml`. Tiene que ser una de las del menú, sin importar mayúsculas. Con la etiqueta `domo-board.icono=<fichero>` lleva además un icono de los del menú común, como `domo-board.icono=libretranslate.svg`; sin ella, su inicial.
- **Otros stacks:** los que no tienen aplicación en el menú ni categoría van en un último grupo, con su inicial. Sin menú configurado, todos los stacks van en un solo grupo.
- **Iconos:** se cargan de `/__menu/iconos/` en el mismo nombre del panel, como en cualquier aplicación con el menú. Si no cargan, por ejemplo al entrar por un túnel SSH, se ve la inicial.

## Actualizar stacks

- **Descubre los stacks:** recorre `stacks/*/docker-compose.yml` en el último commit de la rama, así que un stack nuevo aparece sin darlo de alta.
- **Busca cambios sin aplicarlos:** descarga los commits nuevos con `git fetch` al arrancar, cada 15 minutos y con el botón *Buscar cambios*. Nunca hace `git pull`, así que los ficheros del servidor no cambian hasta que se actualiza un stack.
- **Muestra el estado:** compara el commit de la última actualización correcta de cada stack con el último de la rama, y lista los commits que tocan su carpeta. Avisa de los stacks desplegados que ya no están en el repositorio, sin borrarlos.
- **Actualiza con un botón:** restaura solo la carpeta de ese stack al commit que se veía en la página y lanza `docker compose up -d --remove-orphans --wait`. Un cambio en otro stack no entra en vigor hasta que se actualiza ese stack.
- **En cola:** cada actualización entra en una cola y se aplican de una en una. Se pueden pedir mientras otra está en curso o actualizar a la vez todos los que tienen cambios. Un stack que ya está en cola no se repite.
- **En orden:** los stacks que se piden juntos forman un lote. Dentro del lote van primero los que crean redes que otros del lote usan como externas, según la sección `networks` de cada compose; el resto, por orden alfabético. Si uno falla, se cancelan los siguientes del lote.
- **En un contenedor efímero:** la cola la vacía un contenedor aparte, con la imagen y los montajes del panel y sin red, para que termine aunque el panel se reinicie al actualizarse a sí mismo. Solo hay uno a la vez; el panel lo arranca cuando hay tareas en cola y no está en marcha.
- **Enlaza los servicios publicados:** lee las reglas de los routers de Traefik en las etiquetas de cada compose. `` Host(`nombre.completo`) `` enlaza a ese nombre; `` HostRegexp(`^nombre\.`) `` enlaza a ese nombre con el dominio desde el que se abre el panel. Si el panel se abre por IP o por un túnel SSH, estos últimos no tienen enlace. Las demás reglas se ignoran.
- **Pasa el dominio:** el dominio desde el que se abre el panel al pedir la actualización llega a `docker compose` como la variable `DOMINIO`, para que los stacks lo usen en su configuración (direcciones de enlaces y correos) sin guardarlo en el repositorio. Si el panel se abre por IP o por un túnel SSH, la variable no se pasa.
- **Sin parámetros libres:** tiene acceso al socket de Docker y, por tanto, control total del servidor, así que solo acepta stacks que existan en el repositorio y rechaza formularios enviados desde otras webs.

## Uso

Variables de entorno:

| Variable | Para qué | Por defecto |
| --- | --- | --- |
| `DOMO_REPO` | Ruta del clon del repositorio de despliegue | Obligatoria |
| `DOMO_DATOS` | Carpeta donde guarda las tareas y sus logs | Obligatoria |
| `DOMO_RAMA` | Rama que se despliega | `main` |
| `DOMO_DIRECCION` | Dirección en la que escucha | `:8080` |
| `DOMO_MENU` | Dirección del servicio del menú común en la red interna, para las categorías, los nombres y los iconos | Sin menú: solo los stacks |

Ejemplo de stack, con rutas de ejemplo:

```yaml
services:
  domo-board:
    image: ghcr.io/juanjoi/domo-board:X.Y.Z
    container_name: domo-board
    restart: unless-stopped
    environment:
      - TZ=Europe/Madrid
      - DOMO_REPO=/datos/hub
      - DOMO_DATOS=/datos/domo-board
      - DOMO_MENU=http://domo-menu:8080
    # Solo en el propio servidor: se publica a través de un proxy inverso.
    ports:
      - "127.0.0.1:8080:8080"
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost:8080/salud"]
      interval: 30s
      timeout: 5s
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      # Misma ruta dentro y fuera, para que las rutas de los compose se
      # resuelvan igual que en el servidor.
      - /datos:/datos
```

- **La primera vez** se levanta a mano con `docker compose up -d`. Después se actualiza desde el propio panel.
- **Stacks existentes:** los que se desplegaron antes del panel aparecen como *No desplegado desde el panel* hasta su primera actualización con él.
- **No fijar `hostname`** en el compose: el panel se encuentra a sí mismo por el nombre de su contenedor para copiar su imagen y sus montajes a la tarea.
- **Sin login propio:** quien llegue al panel controla todo el servidor. Conviene que solo sea accesible desde una red de confianza o detrás de un proxy con autenticación.
- **No hacer `git pull` a mano** en el clon: el panel restaura cada stack por separado y mantiene el resto como estaba.
- Si una versión nueva no arranca, se vuelve a la anterior por SSH cambiando la versión en el compose.

## Desarrollo

- Go con la librería estándar, sin dependencias externas. Las pruebas necesitan `git`: `go test ./...`.
- La imagen lleva el binario, `git` y el cliente de Docker con Compose.
- GitHub Actions pasa las pruebas en cada cambio y, al etiquetar una versión `vX.Y.Z`, publica la imagen en GitHub Container Registry. La primera vez hay que hacer pública la imagen en la configuración del paquete, para que el servidor la descargue sin credenciales.
