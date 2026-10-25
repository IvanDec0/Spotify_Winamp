# Documentación técnica --- Cliente musical con skins personalizables

> **Estado:** documento inicial de diseño\
> **Objetivo:** construir una aplicación de escritorio inspirada en
> Winamp (especialmente sus skins clásicos), con una interfaz
> completamente personalizable y un sistema de skins de terceros.\
> **Stack elegido:** Go + Wails v3 + Svelte + TypeScript + CSS.\
> **Plataformas objetivo iniciales:** Linux y Windows.\
> **Idioma de la documentación:** español.

------------------------------------------------------------------------

## 1. Visión del proyecto

La aplicación será un cliente de escritorio para descubrir y controlar
música de Spotify, con una interfaz visual que pueda transformarse por
completo mediante skins instalables.

No se busca únicamente un selector de temas claro/oscuro. Una skin podrá
definir la composición visual del reproductor, la distribución de
paneles, los controles, las fuentes, los recursos gráficos, las
animaciones y, en una etapa posterior, componentes propios.

La referencia conceptual es el ecosistema de skins de Winamp: el núcleo
de la aplicación ofrece capacidades estables y las skins deciden cómo
presentarlas.

### Objetivos

-   Crear una aplicación de escritorio multiplataforma.
-   Separar la lógica del reproductor de la interfaz.
-   Permitir instalar, activar, desactivar, previsualizar y eliminar
    skins.
-   Permitir layouts muy diferentes entre sí, no solo variaciones de
    colores.
-   Definir una API estable para que las skins interactúen con el
    reproductor.
-   Comenzar con una skin propia inspirada en Winamp para validar el
    sistema.
-   Mantener una arquitectura modular sin convertir el proyecto en
    microservicios.

### No objetivos de la primera versión

-   Crear una plataforma de streaming propia.
-   Descargar o extraer audio de Spotify.
-   Ejecutar código arbitrario de cualquier skin sin aislamiento.
-   Construir un editor visual de skins desde el primer día.
-   Implementar sincronización entre dispositivos o una infraestructura
    de cuentas propia.

------------------------------------------------------------------------

## 2. Decisiones tecnológicas

  -----------------------------------------------------------------------
  Área                    Tecnología              Responsabilidad
  ----------------------- ----------------------- -----------------------
  Aplicación de           Wails v3                Ventanas, ciclo de
  escritorio                                      vida, integración con
                                                  el sistema operativo

  Backend                 Go                      Servicios de
                                                  aplicación,
                                                  configuración, archivos
                                                  y coordinación

  Frontend                Svelte + TypeScript     Interfaz, estado visual
                                                  y componentes

  Estilos                 CSS                     Apariencia, animaciones
                                                  y adaptación visual de
                                                  cada skin

  Manifiestos y layouts   JSON                    Metadatos, estructura y
                                                  configuración
                                                  declarativa de skins

  Persistencia local      JSON inicialmente;      Preferencias y
                          SQLite si se necesita   metadatos locales

  Integración musical     Spotify Web API / Web   Catálogo, metadatos y
                          Playback SDK, sujeta a  reproducción permitida
                          validación              

  Pruebas                 Go testing + Vitest     Pruebas del backend y
                                                  frontend
  -----------------------------------------------------------------------

### ¿Por qué Wails v3?

Wails permite escribir la lógica de escritorio en Go y construir la
interfaz con tecnologías web. Se elige v3 deliberadamente, aunque sea
una versión preestable, porque se desea trabajar con su API actual.

**Consecuencia:** fijar una versión concreta de Wails v3, revisar sus
notas de versión antes de actualizar y evitar depender de APIs internas
no documentadas. La documentación oficial consultada describe v3 como
beta y señala que puede seguir evolucionando; por tanto, no asumir que
todas las APIs están congeladas.

Documentación: - Wails v3: https://v3.wails.io/ - Referencia de API:
https://v3.wails.io/reference/overview/ - FAQ y estado del proyecto:
https://v3.wails.io/faq/

### ¿Por qué Svelte?

Svelte permite crear componentes visuales pequeños y mantener el
frontend organizado. TypeScript aporta contratos explícitos para el
modelo del reproductor y el sistema de skins.

No se debe acoplar la lógica de negocio a componentes Svelte concretos.
La interfaz consumirá servicios y un modelo de estado; las skins se
renderizarán a través de un runtime separado.

------------------------------------------------------------------------

## 3. Arquitectura general

``` text
┌──────────────────────────────────────────────────────┐
│                    Aplicación Wails                  │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │             Frontend Svelte / TS               │  │
│  │                                                │  │
│  │  ┌──────────────┐   ┌───────────────────────┐  │  │
│  │  │ Skin Runtime │   │ UI de configuración   │  │  │
│  │  │ + Layout     │   │ y selección de skins  │  │  │
│  │  └──────┬───────┘   └───────────────────────┘  │  │
│  │         │ Skin API                              │  │
│  └─────────┼──────────────────────────────────────┘  │
│            │                                         │
│  ┌─────────▼──────────────────────────────────────┐  │
│  │                 Servicios Go                  │  │
│  │                                                │  │
│  │ SkinService · SettingsService · PlayerService │  │
│  │ SpotifyService · Event/State coordination     │  │
│  └──────────────────────┬─────────────────────────┘  │
│                         │                            │
│  ┌──────────────────────▼─────────────────────────┐  │
│  │ Adaptadores externos y persistencia            │  │
│  │ Spotify · filesystem · configuración local     │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

### Regla arquitectónica principal

**El núcleo define capacidades; las skins definen la presentación.**

El núcleo no debe saber si el botón de pausa se dibuja como un
rectángulo, una imagen pixel-art o un control circular. Solo expone una
operación `pause` y el estado de reproducción correspondiente.

La skin no debe acceder directamente a las credenciales de Spotify, a la
base de datos ni al sistema de archivos. Debe utilizar una API explícita
con permisos limitados.

### Módulos lógicos

-   **Player:** estado y operaciones de reproducción.
-   **Spotify adapter:** autenticación e integración con las API
    admitidas.
-   **Skin manager:** descubrir, validar, instalar, activar y eliminar
    skins.
-   **Skin runtime:** interpretar el manifiesto y montar la interfaz de
    la skin.
-   **Settings:** preferencias y skin activa.
-   **Window manager:** operaciones de ventanas nativas, si se habilitan
    ventanas independientes.
-   **Diagnostics:** logs y errores de carga de skins.

Estos módulos son paquetes dentro de una aplicación modular, no
servicios de red separados.

------------------------------------------------------------------------

## 4. Diseño del sistema de skins

El sistema debe admitir tres niveles de personalización.

### Nivel 1 --- Temas visuales

-   Colores, tipografías, bordes, sombras y fondos.
-   Iconos, imágenes y otros recursos gráficos.
-   Animaciones CSS.
-   Tamaños y espaciados configurables.

### Nivel 2 --- Layouts personalizados

-   Reordenar, ocultar y redimensionar paneles.
-   Definir qué componentes aparecen.
-   Cambiar la distribución de controles.
-   Crear vistas compactas o expandidas.
-   Organizar el reproductor como una ventana o como varias ventanas
    nativas, cuando la plataforma lo permita.

### Nivel 3 --- Componentes de terceros

-   Visualizadores y widgets propios.
-   Componentes Svelte/TypeScript empaquetados con una skin.
-   Interacciones personalizadas mediante la Skin API.
-   Capacidades declaradas y permisos limitados.

**Estrategia recomendada:** implementar los niveles 1 y 2 antes de
ejecutar componentes de terceros. El nivel 3 aumenta mucho la
complejidad y la superficie de ataque.

### Estructura propuesta de una skin

``` text
skins/
└── winamp-classic/
    ├── manifest.json
    ├── layout.json
    ├── styles.css
    ├── assets/
    │   ├── background.png
    │   ├── buttons.png
    │   └── fonts/
    └── components/          # Opcional; etapa avanzada
        └── visualizer/
```

Las skins instaladas se guardarán en el directorio de datos de usuario
de la aplicación, no dentro del directorio de instalación del programa.

### Ejemplo de `manifest.json`

``` json
{
  "schemaVersion": 1,
  "id": "winamp-classic",
  "name": "Winamp Classic",
  "version": "0.1.0",
  "description": "Skin compacta inspirada en reproductores clásicos",
  "author": "Autor de la skin",
  "entry": {
    "layout": "layout.json",
    "styles": "styles.css"
  },
  "capabilities": [
    "player.read",
    "player.control",
    "library.read"
  ]
}
```

Este formato es una propuesta inicial del proyecto, no un estándar
existente. `schemaVersion` permite evolucionar el formato sin romper
skins antiguas.

### Ejemplo de `layout.json`

``` json
{
  "schemaVersion": 1,
  "root": {
    "type": "column",
    "children": [
      {
        "type": "component",
        "component": "titlebar"
      },
      {
        "type": "component",
        "component": "track-info"
      },
      {
        "type": "component",
        "component": "transport-controls"
      },
      {
        "type": "component",
        "component": "playlist"
      }
    ]
  }
}
```

El renderer de layouts deberá admitir únicamente tipos y componentes
registrados. No debe interpretar nombres de componentes arbitrarios como
si fueran instrucciones ejecutables.

### Resolución de recursos

-   Resolver rutas siempre dentro del directorio de la skin.
-   Rechazar rutas que escapen del directorio mediante `..` o enlaces no
    permitidos.
-   Validar tamaño de archivos, formato y versión del manifiesto.
-   Mostrar errores de validación con el nombre de la skin y el archivo
    problemático.
-   No instalar ni activar una skin inválida.

### Instalación y distribución

En la primera versión, una skin puede distribuirse como un directorio o
un archivo ZIP con el manifiesto en la raíz. El instalador deberá
validar el contenido antes de copiarlo al directorio de skins.

No hace falta construir una tienda de skins inicialmente. La instalación
local desde un archivo es suficiente para validar el formato.

------------------------------------------------------------------------

## 5. Skin API

La API debe ser pequeña, estable y orientada a capacidades. La skin no
debe manipular internamente el estado del reproductor: solicita acciones
y recibe actualizaciones.

### Operaciones iniciales

``` ts
export interface PlayerState {
  status: "idle" | "loading" | "playing" | "paused" | "error";
  trackId?: string;
  title?: string;
  artists: string[];
  durationMs?: number;
  positionMs: number;
}

export interface PlayerAPI {
  getState(): Promise<PlayerState>;
  play(): Promise<void>;
  pause(): Promise<void>;
  togglePlayPause(): Promise<void>;
  next(): Promise<void>;
  previous(): Promise<void>;
  seek(positionMs: number): Promise<void>;
  subscribe(
    listener: (state: PlayerState) => void
  ): () => void;
}
```

Es un contrato ilustrativo. Los métodos definitivos deben corresponder a
las operaciones que la integración musical permita realizar.

### Principios de la API

1.  Las operaciones que pueden fallar deben devolver errores explícitos.
2.  Las acciones de control deben validar el estado actual.
3.  La interfaz debe recibir actualizaciones de estado, no consultar el
    reproductor constantemente.
4.  La API debe tener versión y política de compatibilidad.
5.  Una skin solo recibe las capacidades declaradas y concedidas.
6.  El núcleo puede rechazar operaciones no disponibles por el proveedor
    o la cuenta.

### Seguridad de skins avanzadas

Un archivo JavaScript de terceros puede hacer mucho más que dibujar una
interfaz. No debe considerarse seguro por el mero hecho de venir dentro
de una skin.

Antes de habilitar extensiones ejecutables: - Definir un modelo de
permisos. - Aislar el código y limitar las API expuestas. - No exponer
tokens OAuth ni secretos. - Bloquear acceso arbitrario al filesystem,
procesos y red, salvo capacidades expresamente diseñadas. - Considerar
límites de recursos y manejo de errores. - Ofrecer una opción para
deshabilitar skins de terceros.

Para la primera versión, usar JSON + CSS y componentes de la aplicación
es la alternativa más simple y segura.

------------------------------------------------------------------------

## 6. Integración con Spotify: riesgo técnico que hay que validar pronto

La integración con Spotify es una dependencia externa crítica y debe
investigarse antes de construir toda la aplicación alrededor de una
capacidad supuesta.

### Separar el proveedor de música

Definir una interfaz interna para que el resto de la aplicación no
dependa directamente de Spotify:

``` go
type MusicProvider interface {
    GetPlaybackState(ctx context.Context) (PlaybackState, error)
    Play(ctx context.Context) error
    Pause(ctx context.Context) error
    Next(ctx context.Context) error
    Previous(ctx context.Context) error
}
```

Es un ejemplo de diseño: las firmas y los métodos finales dependerán de
las API realmente disponibles.

El frontend no debería comunicarse directamente con múltiples endpoints
de Spotify. Debería llamar al servicio interno, que encapsula
autenticación, errores, límites y diferencias del proveedor.

### Qué comprobar antes de prometer reproducción

-   Qué funciones permite actualmente la Spotify Web API para el tipo de
    aplicación registrado.
-   Requisitos vigentes de acceso, cuotas y Development Mode.
-   Requisitos de cuenta para Web Playback SDK; actualmente requiere
    Spotify Premium.
-   Compatibilidad del SDK con el WebView usado por Wails en cada
    sistema operativo.
-   Restricciones de uso, redistribución y comercialización.
-   Reglas sobre tratamiento del contenido y uso de los datos.

**No asumir que la Web API por sí sola permite reproducir audio
completo.** El Web Playback SDK tiene requisitos propios y sus
condiciones pueden restringir el uso de la integración. La documentación
oficial advierte, entre otras condiciones, que las integraciones de
streaming no pueden ser comerciales y que el contenido no puede
alterarse.

Fuentes oficiales: - Spotify Web API:
https://developer.spotify.com/documentation/web-api - Web Playback SDK:
https://developer.spotify.com/documentation/web-playback-sdk - Tutorial
de Web Playback SDK:
https://developer.spotify.com/documentation/web-playback-sdk/tutorials/getting-started -
Spotify Developer Policy: https://developer.spotify.com/policy

### Decisión de implementación

Primero desarrollar un **modo demo con datos simulados**. Esto permite
construir y probar el reproductor visual, los layouts y las skins sin
bloquear el desarrollo por autenticación, cuotas o compatibilidad del
SDK.

La interfaz debe representar estados no disponibles o errores del
proveedor sin romper el runtime de skins.

------------------------------------------------------------------------

## 7. Estructura inicial del repositorio

La estructura concreta deberá ajustarse a lo que genere el template
vigente de Wails v3. No sobrescribir archivos de arranque generados sin
comprobar sus responsabilidades.

Una estructura objetivo posible:

``` text
music-client/
├── main.go
├── go.mod
├── Taskfile.yml
├── internal/
│   ├── app/
│   ├── player/
│   ├── musicprovider/
│   │   ├── provider.go
│   │   └── spotify/
│   ├── skins/
│   │   ├── manifest.go
│   │   ├── validator.go
│   │   ├── installer.go
│   │   └── manager.go
│   ├── settings/
│   └── diagnostics/
├── frontend/
│   ├── src/
│   │   ├── app/
│   │   ├── components/
│   │   ├── player/
│   │   ├── skins/
│   │   │   ├── runtime/
│   │   │   ├── layout/
│   │   │   └── api/
│   │   └── settings/
│   └── package.json
├── skins-dev/
│   └── winamp-classic/
└── docs/
    ├── architecture.md
    ├── skin-format.md
    └── skin-api.md
```

`skins-dev/` contiene skins para desarrollo y no necesariamente debe
distribuirse dentro del ejecutable. La ubicación final de las skins
instaladas será un directorio de datos de usuario obtenido mediante las
APIs apropiadas de la aplicación.

------------------------------------------------------------------------

## 8. Plan de desarrollo por etapas

### Etapa 0 --- Validar herramientas y proveedor

-   [ ] Instalar los prerrequisitos indicados por la documentación
    actual de Wails v3.
-   [ ] Ejecutar `wails3 doctor`.
-   [ ] Crear un proyecto usando el template Svelte + TypeScript
    disponible en la versión instalada.
-   [ ] Ejecutar el modo de desarrollo y comprobar la comunicación
    Go--frontend.
-   [ ] Revisar las limitaciones actuales de Spotify API y Web Playback
    SDK.
-   [ ] Registrar las versiones de Go, Wails, Node.js y dependencias.

### Etapa 1 --- Esqueleto funcional

-   [ ] Crear la ventana principal.
-   [ ] Definir el modelo `PlayerState`.
-   [ ] Implementar un `MusicProvider` simulado.
-   [ ] Mostrar tema, artista, estado y posición ficticios.
-   [ ] Implementar play, pausa, siguiente y anterior simulados.
-   [ ] Probar eventos de actualización desde Go al frontend.

**Criterio de salida:** la aplicación se ejecuta y el reproductor
funciona en modo demo.

### Etapa 2 --- Primera skin personalizada

-   [ ] Definir `manifest.json` y validarlo.
-   [ ] Implementar un layout renderer con componentes registrados.
-   [ ] Cargar CSS y assets locales de la skin.
-   [ ] Crear `winamp-classic`.
-   [ ] Separar el estado del reproductor de la presentación.
-   [ ] Mostrar errores útiles ante manifiestos inválidos.

**Criterio de salida:** la skin modifica de forma visible el layout y la
estética sin cambiar el código del núcleo.

### Etapa 3 --- Skin Manager

-   [ ] Detectar skins instaladas.
-   [ ] Activar y desactivar skins.
-   [ ] Recordar la skin activa.
-   [ ] Importar skins desde ZIP con validación.
-   [ ] Previsualizar una skin antes de activarla.
-   [ ] Revertir a una skin predeterminada si la seleccionada falla.

**Criterio de salida:** un usuario puede instalar una segunda skin y
alternar entre ambas sin recompilar la aplicación.

### Etapa 4 --- Layouts realmente distintos

-   [ ] Soportar layouts de una o varias columnas.
-   [ ] Permitir ocultar y reordenar componentes admitidos.
-   [ ] Definir tamaños mínimos y comportamiento responsive.
-   [ ] Gestionar persistencia del tamaño de ventana.
-   [ ] Evaluar ventanas nativas independientes.
-   [ ] Probar skins con resoluciones y escalas distintas.

**Criterio de salida:** dos skins pueden organizar el reproductor de
forma radicalmente distinta.

### Etapa 5 --- Integración real con Spotify

-   [ ] Registrar y configurar la aplicación de Spotify.
-   [ ] Implementar el flujo de autenticación admitido.
-   [ ] Guardar tokens de forma segura y gestionar su expiración.
-   [ ] Implementar el adaptador de proveedor.
-   [ ] Validar reproducción, controles y estado en los sistemas
    objetivo.
-   [ ] Gestionar errores de permisos, cuenta, red y límites de API.

**Criterio de salida:** la integración real funciona dentro de las
capacidades y condiciones permitidas por Spotify.

### Etapa 6 --- Extensiones programables

-   [ ] Definir una versión estable de la Skin API.
-   [ ] Establecer permisos y aislamiento para componentes externos.
-   [ ] Definir límites de consumo y errores.
-   [ ] Documentar el formato para autores de skins.
-   [ ] Añadir ejemplos y pruebas de compatibilidad.
-   [ ] Implementar distribución y actualización solo cuando el formato
    sea estable.

No empezar esta etapa hasta que el sistema declarativo de skins funcione
correctamente.

------------------------------------------------------------------------

## 9. Pruebas y criterios de calidad

### Backend Go

-   Validación de manifiestos correctos e incorrectos.
-   Rechazo de rutas que intenten salir del directorio de la skin.
-   Instalación y eliminación sin sobrescribir archivos ajenos.
-   Gestión de errores del proveedor musical.
-   Persistencia y recuperación de configuración.

### Frontend

-   Montaje y desmontaje de skins.
-   Manejo de acciones y actualizaciones de estado.
-   Ausencia de listeners duplicados al cambiar de skin.
-   Comportamiento con layouts incompletos o inválidos.
-   Diseño usable en ventanas pequeñas y grandes.

### Integración

-   Cambiar de skin durante la reproducción no debe detenerla.
-   Un error al cargar una skin debe permitir volver a la
    predeterminada.
-   Una skin no puede cambiar las reglas del núcleo.
-   La aplicación debe seguir siendo usable cuando Spotify no está
    disponible.

------------------------------------------------------------------------

## 10. Decisiones que conviene mantener

1.  **Primero una aplicación modular, no microservicios.** No hay
    necesidad de separar procesos ni añadir gRPC para este alcance
    inicial.
2.  **El núcleo no depende de la skin.** La skin consume interfaces
    estables.
3.  **Modo demo antes de integración real.** La interfaz y el sistema de
    skins deben poder desarrollarse sin depender de Spotify.
4.  **JSON + CSS antes de código ejecutable.** Es más sencillo de
    validar, probar y mantener.
5.  **Versionar el formato de skins.** No confundir la versión del
    formato con la versión de la aplicación.
6.  **Validar las restricciones de Spotify temprano.** No construir
    funcionalidades sobre capacidades no autorizadas.
7.  **Fijar versiones de Wails v3.** Al ser una versión preestable, las
    actualizaciones deben ser deliberadas.
8.  **Diseñar para fallos.** Una skin defectuosa nunca debe impedir que
    la aplicación arranque.

------------------------------------------------------------------------

## 11. Primeros comandos

Consultar primero las instrucciones actuales de instalación de Wails v3:

https://v3.wails.io/

La documentación oficial presenta este flujo inicial:

``` bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
wails3 setup
wails3 doctor
wails3 init -n music-client -t svelte-ts
cd music-client
wails3 dev
```

**Importante:** los nombres de los templates y los comandos pueden
cambiar entre versiones de Wails v3. Si `svelte-ts` no aparece en la
versión instalada, consultar los templates disponibles en la CLI y
elegir el template Svelte con TypeScript. No continuar copiando comandos
de documentación de Wails v2: las CLI y APIs no son intercambiables.

No es necesario ejecutar todos los comandos a ciegas. Primero comprobar
los prerrequisitos y el resultado de `wails3 doctor`.

------------------------------------------------------------------------

## 12. Referencias oficiales

### Wails v3

-   Inicio: https://v3.wails.io/
-   Referencia API: https://v3.wails.io/reference/overview/
-   FAQ y estado: https://v3.wails.io/faq/
-   Repositorio: https://github.com/wailsapp/wails

### Frontend

-   Svelte: https://svelte.dev/
-   TypeScript: https://www.typescriptlang.org/docs/
-   MDN CSS: https://developer.mozilla.org/docs/Web/CSS

### Spotify

-   Web API: https://developer.spotify.com/documentation/web-api
-   Web Playback SDK:
    https://developer.spotify.com/documentation/web-playback-sdk
-   Política para desarrolladores: https://developer.spotify.com/policy

------------------------------------------------------------------------

## 13. Definición de la primera versión utilizable

La primera versión no necesita reproducir música real para validar la
arquitectura. Se considera un primer hito útil cuando:

1.  La aplicación arranca con Wails v3.
2.  El reproductor demo muestra una canción ficticia y responde a sus
    controles.
3.  Existe una skin predeterminada.
4.  Se puede instalar una segunda skin desde un directorio o ZIP.
5.  Cambiar de skin cambia el layout y la apariencia sin recompilar.
6.  Un error de skin no rompe el núcleo ni impide recuperar la interfaz.
7.  La documentación del formato permite crear una skin nueva sin
    modificar el backend.

**Prioridad inmediata:** crear el esqueleto Wails v3 +
Svelte/TypeScript, implementar el proveedor simulado y definir el
contrato de `manifest.json` y `layout.json`. No comenzar por OAuth ni
por el sistema de extensiones ejecutables.
