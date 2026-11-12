# webscope

[English](README.md)

Un servidor MCP que deja a un agente de IA probar una aplicación web y entender sus datos y su código gastando muy pocas fichas. Lee tu base de datos, mira y maneja una pantalla en tu Chrome, comprueba que lo que una pantalla envió es lo que quedó guardado y lee el código del proyecto. Un binario de Go, cuatro herramientas, y nunca escribe en tu base de datos.

## Para qué sirve

Cuando le pides a un agente «prueba la pantalla de cotizaciones», lo caro no es pensar: es leer. Páginas enteras de HTML, cientos de filas de una tabla, la salida de un `grep` sobre un backend. webscope responde con texto corto, acotado y estructurado, así que el agente puede hacer la vuelta completa (mirar la pantalla, llenarla, guardar, ver qué se guardó y qué más cambió) en unas pocas llamadas.

Se construyó contra una aplicación real Laravel + Vue y su versión anterior en PHP, y funciona con cualquier aplicación web y cualquier base MySQL o SQLite. Algunas piezas están atadas a un stack: `map`, `map_back`, `contract`, `flows` y `cases` a las rutas de Laravel y los módulos de Vue, y `legacy` a PHP antiguo con el SQL dentro del archivo y llamadas jQuery; el resto es genérico.

## Requisitos

- Go 1.27.3 o más nuevo, para compilarlo (`go.mod`).
- Un cliente MCP. El arranque rápido de abajo usa Claude Code.
- Para `query`, `check` y `code fields`: una base MySQL o SQLite.
- Para `page` y `check`: Google Chrome con la extensión Claude in Chrome y una pestaña abierta en la aplicación que pruebas.
- Para `code`: las carpetas del código de la aplicación (un backend Laravel y un front Vue para `map`, `map_back`, `flows` y `cases`).

## Qué le puedes pedir a tu agente

Con el servidor registrado, pide en palabras normales:

- «¿Cuántas filas tiene `quotes` y qué columnas están siempre vacías?» (`query`, `code fields`)
- «¿Quién lee o escribe la columna `status` de `quotes` en el backend?» (`code consumers`)
- «Abre la pantalla de cotizaciones, llena el formulario con datos de prueba, guárdalo y dime si lo que envió la pantalla es lo que se guardó.» (`page`, `check`)
- «¿Qué escribió de verdad guardar esa cotización? Lista cada fila que tocó, con las tablas hijas.» (`code snap`, `snap_diff`)
- «Haz que la lista de cotizaciones responda un 500 y dime qué muestra la pantalla.» (`page simulate`)
- «Audita esta pantalla: texto sin renderizar, controles tapados, campos sin nombre.» (`page audit`)
- «¿Qué envía la pantalla vieja cuando pulso Buscar?» (`page payloads`, también en páginas antiguas con jQuery)
- «¿La pantalla nueva envía y recibe los mismos campos que la vieja?» (`page parity {req_a, req_b}`)
- «Compara la versión vieja y la nueva de esta pantalla.» (`page shape` en cada una y después `page parity`)
- «¿Qué endpoint, controlador y tabla usa esta pantalla?» (`code map_back`)
- «¿Esta pantalla manda lo que el endpoint valida?» (`code contract`)
- «¿Cuáles son los flujos y las historias de usuario de este módulo?» (`code flows`; las historias las escribes a partir de eso y las confirmas)
- «¿Qué debo probar en este módulo, y la migración conserva lo que hacía el sistema viejo?» (`code cases` y después la cadena de «Migrar un módulo», abajo)
- «¿Qué pruebas unitarias fijan lo que hacía el sistema viejo?» (paso 7 de `code help {topic: "migration"}`)

## Las cuatro herramientas

| Herramienta | En una línea | Necesita |
|---|---|---|
| `query` | Una sentencia SQL de solo lectura; la respuesta es una tabla corta. | `WEBSCOPE_DB_URL` |
| `page` | Leer, llenar y pulsar en una pantalla de tu Chrome, ver las peticiones que envió, forzar un error HTTP, auditarla y comparar una pantalla vieja con una nueva. | Claude in Chrome y `WEBSCOPE_APP_URL` |
| `check` | Para una fila guardada: lo que envió la pantalla, lo que guardó la base y lo que la pantalla volvió a leer. Solo las diferencias. | `page` y la base |
| `code` | La ficha de una tabla, quién usa una columna, las rutas y llamadas de una pantalla, qué cambió una acción, los flujos y casos de prueba de un módulo, y qué llama y escribe un módulo PHP antiguo. | `WEBSCOPE_BACKEND`, `WEBSCOPE_FRONT`, `WEBSCOPE_LEGACY` |

Una sesión típica, en el orden que mejor funciona (`docs/usage.md` tiene el contrato de cada herramienta):

1. `code fields` y `query`: conocer la tabla y sus conteos actuales.
2. `code map` o `code map_back`: saber qué endpoints y tablas toca la pantalla.
3. `code snap`: guardar la tabla como está antes de la acción.
4. `page`: `load`, `state`, `fill`, `click`.
5. `check`: comparar la fila guardada con lo que se envió y se volvió a leer.
6. `code snap_diff`: ver cada fila que escribió la acción.
7. Limpiar por la pantalla, nunca por la base.

## Instalar como plugin de Claude Code

    claude plugin marketplace add partprogramming09/webscope
    claude plugin install webscope@webscope

Después, `/plugin configure webscope@webscope` pide la URL de la base (se guarda como valor sensible) y los demás ajustes; uno vacío deja sin su herramienta. No pases la URL por la línea de comandos: queda en el historial del shell y se ve en `ps`.

Trae el servidor y las cuatro skills. El primer arranque compila el binario (hace falta Go; de 8 a 14 s según tenga Go los módulos ya descargados; `bin/webscope-launch build` lo hace antes). Otros agentes (Codex, Cursor, VS Code, Gemini CLI) y el detalle: [docs/install.md](docs/install.md) (en inglés).

## Arranque rápido (Claude Code)

    go build -o webscope .        # Go 1.27.3 o más nuevo (go.mod)
    claude mcp add webscope \
      -e WEBSCOPE_DB_URL='mysql://user:pass@127.0.0.1:3306/dbname' \
      -e WEBSCOPE_APP_URL='http://localhost:5173' \
      -e WEBSCOPE_BACKEND='/path/to/backend' \
      -e WEBSCOPE_FRONT='/path/to/front' \
      -- /absolute/path/to/webscope

Para empezar solo hace falta `WEBSCOPE_DB_URL` (`query`). Las demás encienden el resto:

- `WEBSCOPE_APP_URL`: la dirección de la pestaña que se prueba, tal como la escribe el navegador (esquema, host y puerto). La necesita `page`, que además necesita la extensión Claude in Chrome. Si la aplicación vieja y la nueva responden en direcciones distintas, ponlas separadas por comas (`http://localhost:8088,http://demo.localhost:5173`): el ayudante funciona en una pestaña de cualquiera de ellas y las peticiones de todas se guardan juntas.
- `WEBSCOPE_BACKEND`: la carpeta con el PHP del proyecto. La usa `code`.
- `WEBSCOPE_FRONT`: la carpeta del front Vue (se lee su `src/modules` si existe). La usan `code map`, `map_back` y `contract`.
- `WEBSCOPE_LEGACY`: la carpeta del PHP antiguo (la que contiene las carpetas de los módulos, por ejemplo `public_html`). La lee `code legacy`; nunca sale de ella, no sigue enlaces y no abre `.env*`.
- `WEBSCOPE_TENANT_QUERY`: una búsqueda de tenant distinta de la de Hyn Tenancy (ver la referencia).
- `WEBSCOPE_DOWNLOADS`: la carpeta donde Chrome guarda los archivos, para `page download` (por defecto `~/Downloads`).
- `WEBSCOPE_LOG`: el log del backend (por ejemplo el log de errores de PHP): cuando una petición termina con estado 0 o 5xx, `click`, `press` y `payloads` añaden las líneas de error escritas desde que empezó ese clic o tecla (hasta 3, de 200 caracteres cada una, tal como las escribió el log: pueden llevar datos). El ayudante anota dónde termina el archivo antes de cada clic o tecla, así que no necesita reloj ni formato de fecha. `payloads {after: N}` añade las líneas de error escritas desde que `payloads` respondió `mark N`, sea cual sea el estado; así se lee lo que hizo un clic dado con la herramienta propia del navegador.

Después de cambiar una variable o de recompilar, reconecta con `/mcp`: el cliente conectado sigue hablando con el proceso viejo. Para comprobar que funciona, pide al agente `query` con `SELECT 1` y `page(help)`.

## Qué garantiza

- **Nunca escribe en la base.** `query` es de solo lectura dos veces (una comprobación de la sentencia y una sesión de solo lectura); `check`, `code` y `snap` solo leen.
- **Las escrituras pasan solo por la pantalla**, con tu aprobación, en filas con el prefijo `ZZ_`. La limpieza también se hace por la pantalla.
- **Las respuestas son pequeñas y dicen lo que dejaron fuera**: 50 filas (como mucho 500), 200 caracteres por celda, 8000 bytes por respuesta. La lista de herramientas que carga el agente se mantiene por debajo de 1800 bytes (lo comprueba un test).
- **`page` es local**: un servidor en `127.0.0.1` con puerto y token aleatorios, que solo responde al origen de tu aplicación.
- **Sin adivinar**: `check` recibe de ti qué campo va en qué columna, y toda respuesta de `code` que sale de buscar texto dice que es una estimación.
- Con datos de clientes, pide el esquema y conteos, no filas.
- **La herramienta informa, el modelo decide**: webscope solo etiqueta hechos que puede comprobar y nunca bloquea una acción. Cómo informar de una prueba con honestidad (lo verificado separado de lo no verificado, un interceptor dicho en la primera línea, una descarga que solo cuenta si el archivo está en disco) está en [docs/usage.md](docs/usage.md#reporting-a-test).

## Lo que conviene aprovechar

- **Tenants.** `tenant` en `query`, `check` y `code fields` ejecuta en otra base, que se busca en la central (Hyn Tenancy por defecto).
- **Direcciones correctas.** `page open {path}` va a una ruta del router que dio `code map` a través del propio Vue Router de la aplicación (que añade el prefijo del tenant), y rechaza una ruta que ninguna ruta del router tiene, nombrando las parecidas, así que nunca se llega a una pantalla escribiendo la URL a mano.
- **Errores a pedido.** `page simulate {match, status}` hace que las siguientes peticiones a una URL respondan un error HTTP, para ver qué hace la pantalla con un 500 sin romper nada.
- **¿Qué escribió de verdad la acción?** `snap` antes y `snap_diff` después muestran solo lo que cambió, con las filas hijas.
- **También páginas viejas.** El ayudante ve `fetch` y `XMLHttpRequest`, incluso en páginas que reemplazan el constructor (las páginas PHP antiguas de la aplicación que motivó esto); un enlace antiguo recarga la página, así que hay que volver a ejecutar `page(load)` después de cada uno.
- **Revisar una pantalla sin referencia.** `page audit` lista texto sin renderizar, controles tapados, campos sin etiqueta, etiquetas repetidas y botones sin nombre; es una pista, no un veredicto.
- **Una lista de lo encontrado.** `docs/patterns.md` recoge los hallazgos al probar pantallas, cada uno con cómo reconocerlo, qué hacer y hasta dónde se verificó. `docs/module-testing.md` es el procedimiento para probar un módulo entero.

## Migrar un módulo desde un sistema antiguo

El caso para el que se amplió: un módulo que existe en un sistema viejo y se rehízo, y quieres saber qué le falta, qué añade o qué cambia el nuevo. Hacen falta dos orígenes (`WEBSCOPE_APP_URL=http://old:8088,http://new:5173`), la carpeta del sistema viejo (`WEBSCOPE_LEGACY`) y dos pestañas de Chrome con la sesión iniciada. Todo lo de abajo lee; lo único que se escribe es una fila de prueba por sistema, por la pantalla, con el prefijo `ZZ_` y solo con tu aprobación.

| Paso | Qué pides | Qué ejecuta | Qué devuelve |
|---|---|---|---|
| 1. Qué hace el módulo | «Dame los flujos y las historias de usuario de `crm/quotes`» | `code flows` | Pantallas y acciones con sus condiciones y permisos, y después endpoints, controladores y tablas, como evidencia; las historias se escriben a partir de eso y tú las confirmas |
| 2. Qué toca el código viejo | «¿Qué llama y escribe el módulo viejo de cotizaciones, y qué tablas no tiene el nuevo?» | `code legacy {path, module}` | Cada endpoint viejo con quién lo llama y sus tablas; las tablas que solo tiene un lado, separadas en «nombradas en otra parte del backend» y «no nombradas» |
| 3. Pantallas | «Compara la lista de cotizaciones de la pantalla vieja y la nueva» | `page shape {name, selector}` en cada una, `page parity {a, b}` | Solo los campos, botones y columnas que tiene un lado |
| 4. Peticiones | «¿La búsqueda nueva envía y recibe lo mismo que la vieja?» | la misma acción en las dos, `page parity {req_a, req_b, map}` | El estado y los nombres de campo que solo envía o recibe un lado, nunca valores |
| 5. Qué probar | «Lista los casos de prueba de este módulo» | `code cases` | Una tabla: camino feliz, permiso, campo obligatorio, estado, error HTTP y lista vacía, con lo que ejecuta cada uno |
| 6. Una escritura en los dos | «Crea una cotización `ZZ_` en cada sistema y compara lo que se guardó» | `code snap`, `page fill` y `click`, `code snap_diff`, `code rows`, `query` | Las filas que escribió cada sistema, sus diferencias y las filas hijas por conteo |
| 7. Pruebas unitarias | «Planifica las pruebas unitarias que fijan el comportamiento viejo» | `code help {topic: "migration"}`, paso 7 | Cómo escribir cada diferencia confirmada como prueba unitaria, en la capa dueña de la regla |

La cadena completa, paso a paso, es `code help {topic: "migration"}`. Una corrida real sobre un módulo de cotizaciones encontró, entre otras cosas: 21 de 22 tablas «solo en el sistema viejo» en realidad las usaba el backend nuevo, una vista de la base que lee el código viejo no existe, la pantalla vieja guardaba un número que la nueva ya había tomado (`Duplicate entry … 1062`), y los dos sistemas escribían las mismas filas hijas para una cotización nueva. `CHANGELOG.md` tiene el detalle de cada corrida y de lo que no se comprobó.

## Referencia

La referencia de los argumentos de cada herramienta (`query`, `page`, `check` y `code`), lo medido en una aplicación real, los límites conocidos y lo que no está en esta versión están en inglés, en [README.md](README.md#reference). Esa parte cambia en cada versión y se mantiene en un solo idioma para que no se desfase.

## Documentación

`docs/usage.md` es el contrato de cada herramienta y el orden en que se usan; `docs/module-testing.md`, el procedimiento para probar un módulo, y `docs/patterns.md`, la lista de hallazgos al probar pantallas. `CHANGELOG.md` tiene una entrada por versión. Para compilar, probar o cambiar el servidor, ver [CONTRIBUTING.md](CONTRIBUTING.md); para avisar de una vulnerabilidad, [SECURITY.md](SECURITY.md). Los documentos de `docs/` están en inglés.
