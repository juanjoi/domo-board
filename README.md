# Panel del hub domótico

Aplicación web pequeña para gestionar un hub domótico basado en Docker Compose. Se despliega como un stack más y lee los stacks de un repositorio de despliegue clonado en el servidor.

## Funciones

| Función | Estado |
| --- | --- |
| Ver los stacks y si tienen cambios sin desplegar | Hecho |
| Actualizar un stack con un botón, incluido el propio panel | Hecho |
| Activar y desactivar aplicaciones | Más adelante |

## Actualizar stacks

- **Descubre los stacks:** recorre `stacks/*/docker-compose.yml` en el último commit de la rama, así que un stack nuevo aparece sin darlo de alta.
- **Busca cambios sin aplicarlos:** descarga los commits nuevos con `git fetch` al arrancar, cada 15 minutos y con el botón *Buscar cambios*. Nunca hace `git pull`, así que los ficheros del servidor no cambian hasta que se actualiza un stack.
- **Muestra el estado:** compara el commit de la última actualización correcta de cada stack con el último de la rama, y lista los commits que tocan su carpeta. Avisa de los stacks desplegados que ya no están en el repositorio, sin borrarlos.
- **Actualiza con un botón:** restaura solo la carpeta de ese stack al commit que se veía en la página y lanza `docker compose up -d --remove-orphans --wait`. Un cambio en otro stack no entra en vigor hasta que se actualiza ese stack.
- **En un contenedor efímero:** cada actualización corre en un contenedor aparte, con la imagen y los montajes del panel y sin red, para que termine aunque el panel se reinicie al actualizarse a sí mismo. Solo corre una tarea a la vez.
- **Sin parámetros libres:** tiene acceso al socket de Docker y, por tanto, control total del servidor, así que solo acepta stacks que existan en el repositorio y rechaza formularios enviados desde otras webs.

## Uso

Variables de entorno:

| Variable | Para qué | Por defecto |
| --- | --- | --- |
| `DOMO_REPO` | Ruta del clon del repositorio de despliegue | Obligatoria |
| `DOMO_DATOS` | Carpeta donde guarda las tareas y sus logs | Obligatoria |
| `DOMO_RAMA` | Rama que se despliega | `main` |
| `DOMO_DIRECCION` | Dirección en la que escucha | `:8080` |

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
    # Solo en el propio servidor: se publica a través de un proxy con autenticación.
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
- **Sin login propio:** el panel controla todo el servidor, así que debe quedar detrás de un proxy que pida usuario y contraseña, o accesible solo desde el propio servidor.
- **No hacer `git pull` a mano** en el clon: el panel restaura cada stack por separado y mantiene el resto como estaba.
- Si una versión nueva no arranca, se vuelve a la anterior por SSH cambiando la versión en el compose.

## Desarrollo

- Go con la librería estándar, sin dependencias externas. Las pruebas necesitan `git`: `go test ./...`.
- La imagen lleva el binario, `git` y el cliente de Docker con Compose.
- GitHub Actions pasa las pruebas en cada cambio y, al etiquetar una versión `vX.Y.Z`, publica la imagen en GitHub Container Registry. La primera vez hay que hacer pública la imagen en la configuración del paquete, para que el servidor la descargue sin credenciales.
