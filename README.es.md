# webscope

[![go](https://github.com/partprogramming09/webscope/actions/workflows/go.yml/badge.svg)](https://github.com/partprogramming09/webscope/actions/workflows/go.yml)

[English](README.md)

Un servidor MCP que deja a un agente de IA probar una aplicación web y entender sus datos y su código gastando muy pocas fichas. Lee tu base de datos, mira y maneja una pantalla en tu Chrome, comprueba que lo que una pantalla envió es lo que quedó guardado y lee el código del proyecto. Un solo binario de Go, cuatro herramientas, y nunca escribe en tu base de datos.

## Por qué sirve

Cuando le pides a un agente «prueba la pantalla de cotizaciones», lo caro no es pensar: es leer. Páginas enteras de HTML, cientos de filas de una tabla, la salida de un `grep` sobre un backend. webscope responde con texto corto, acotado y estructurado, así que el agente puede hacer la vuelta entera (mirar la pantalla, llenarla, guardar, ver qué se guardó, ver qué más cambió) en pocas llamadas.

Funciona con cualquier aplicación web y cualquier base MySQL o SQLite. Se construyó contra una aplicación real Laravel + Vue y su versión anterior en PHP, así que algunas piezas están atadas a un stack: `map`, `map_back`, `contract`, `flows` y `cases` a las rutas de Laravel y los módulos de Vue, y `legacy` a PHP antiguo con el SQL dentro del archivo y llamadas jQuery. El resto es genérico.

## Cómo funciona

1. Tu agente (Claude Code, o cualquier cliente MCP) arranca webscope como un programa local y habla con él por stdio.
2. webscope responde desde tres sitios: tu base de datos (solo lectura), tu Chrome (le da al agente un fragmento corto de JavaScript para que lo ejecute; no maneja el navegador por sí mismo) y las carpetas del código de tu proyecto (solo lectura).
3. Cada respuesta es corta y dice lo que dejó fuera. El agente decide qué significan los hallazgos; webscope solo informa de hechos que puede comprobar.

## Qué le puedes pedir a tu agente

Con el servidor registrado, pide en palabras normales:

- «¿Cuántas filas tiene `quotes` y qué columnas están siempre vacías?» (`query`, `code fields`)
- «Abre la pantalla de cotizaciones, llena el formulario con datos de prueba, guárdalo y dime si lo que envió la pantalla es lo que se guardó.» (`page`, `check`)
- «¿Qué escribió de verdad guardar esa cotización? Lista cada fila que tocó, con las tablas hijas.» (`code snap`, `snap_diff`)
- «Haz que la lista de cotizaciones responda un 500 y dime qué muestra la pantalla.» (`page simulate`)
- «Audita esta pantalla: texto sin renderizar, controles tapados, campos sin nombre.» (`page audit`)
- «¿Qué endpoint, controlador y tabla usa esta pantalla, y manda lo que el endpoint valida?» (`code map_back`, `code contract`)
- «¿La pantalla nueva envía y recibe los mismos campos que la vieja?» (`page shape`, `page parity`)
- «¿Cuáles son los flujos de este módulo, qué debo probar y la migración conserva lo que hacía el sistema viejo?» (`code flows`, `code cases` y «Migrar un módulo», abajo)

## Las cuatro herramientas

| Herramienta | En una línea | Necesita |
|---|---|---|
| `query` | Una sentencia SQL de solo lectura; la respuesta es una tabla corta. | `WEBSCOPE_DB_URL` |
| `page` | Leer, llenar y pulsar en una pantalla de tu Chrome, ver las peticiones que envió, forzar un error HTTP, auditarla y comparar una pantalla vieja con una nueva. | Claude in Chrome y `WEBSCOPE_APP_URL` |
| `check` | Para una fila guardada: lo que envió la pantalla, lo que guardó la base y lo que la pantalla volvió a leer. Solo las diferencias. | `page` y la base |
| `code` | La ficha de una tabla, quién usa una columna, las rutas y llamadas de una pantalla, qué cambió una acción, los flujos y casos de prueba de un módulo, y qué llama y escribe un módulo PHP antiguo. | `WEBSCOPE_BACKEND`, `WEBSCOPE_FRONT`, `WEBSCOPE_LEGACY` |

Una sesión típica, en el orden que mejor funciona:

1. `code fields` y `query`: conocer la tabla y sus conteos actuales.
2. `code map` o `code map_back`: saber qué endpoints y tablas toca la pantalla.
3. `code snap`: guardar la tabla como está antes de la acción.
4. `page`: `load`, `state`, `fill`, `click`.
5. `check`: comparar la fila guardada con lo que se envió y se volvió a leer.
6. `code snap_diff`: ver cada fila que escribió la acción.
7. Limpiar por la pantalla, nunca por la base.

## Primeros pasos

### Requisitos

- Un cliente MCP. Los pasos de abajo usan Claude Code; [docs/install.md](docs/install.md) (en inglés) cubre Codex, Cursor, VS Code y Gemini CLI.
- Go 1.27.3 o más nuevo, para compilar el servidor (`go.mod`).
- Para `query`, `check` y `code fields`: una base MySQL o SQLite.
- Para `page` y `check`: Google Chrome con la extensión Claude in Chrome y una pestaña abierta en la aplicación que pruebas.
- Para `code`: las carpetas del código de la aplicación (un backend Laravel y un front Vue para `map`, `map_back`, `flows` y `cases`).

### Instalar como plugin de Claude Code

    claude plugin marketplace add partprogramming09/webscope
    claude plugin install webscope@webscope

Después, `/plugin configure webscope@webscope` pide la URL de la base (se guarda como valor sensible) y los demás ajustes; uno vacío deja sin su herramienta. No pases la URL por la línea de comandos: queda en el historial del shell y se ve en `ps`. El plugin trae el servidor y cuatro skills. El primer arranque compila el binario (de 8 a 14 s según tenga Go los módulos ya descargados; `bin/webscope-launch build` lo hace antes).

### O compilarlo tú

    go build -o webscope .
    claude mcp add webscope \
      -e WEBSCOPE_DB_URL='mysql://user:pass@127.0.0.1:3306/dbname' \
      -e WEBSCOPE_APP_URL='http://localhost:5173' \
      -e WEBSCOPE_BACKEND='/path/to/backend' \
      -e WEBSCOPE_FRONT='/path/to/front' \
      -- /absolute/path/to/webscope

Para empezar solo hace falta `WEBSCOPE_DB_URL` (`query`); cada una de las otras variables enciende una herramienta más. Todas, con lo que hace cada una, están en [docs/reference.md](docs/reference.md#environment-variables) (en inglés).

### Comprobar que funciona

Reconecta con `/mcp` (el cliente conectado sigue hablando con el proceso viejo) y pide al agente `query` con `SELECT 1` y `page(help)`.

## Qué garantiza

- **Nunca escribe en la base.** `query` es de solo lectura dos veces (una comprobación de la sentencia y una sesión de solo lectura); `check`, `code` y `snap` solo leen.
- **Las escrituras pasan solo por la pantalla**, con tu aprobación, en filas con el prefijo `ZZ_`. La limpieza también se hace por la pantalla.
- **Las respuestas son pequeñas y dicen lo que dejaron fuera**: 50 filas (como mucho 500), 200 caracteres por celda, 8000 bytes por respuesta. La lista de herramientas que carga el agente se mantiene por debajo de 1800 bytes (lo comprueba un test).
- **`page` es local**: un servidor en `127.0.0.1` con puerto y token aleatorios, que solo responde al origen de tu aplicación.
- **Sin adivinar**: `check` recibe de ti qué campo va en qué columna, y toda respuesta de `code` que sale de buscar texto dice que es una estimación.
- Con datos de clientes, pide el esquema y conteos, no filas.
- **La herramienta informa, el modelo decide**: webscope solo etiqueta hechos que puede comprobar y nunca bloquea una acción. Cómo informar de una prueba con honestidad está en [docs/usage.md](docs/usage.md#reporting-a-test) (en inglés).

## Migrar un módulo desde un sistema antiguo

El caso para el que se amplió: un módulo que existe en un sistema viejo y se rehízo, y quieres saber qué le falta, qué añade o qué cambia el nuevo. Hacen falta dos orígenes (`WEBSCOPE_APP_URL=http://old:8088,http://new:5173`), la carpeta del sistema viejo (`WEBSCOPE_LEGACY`) y dos pestañas de Chrome con la sesión iniciada. Todo lo de abajo lee; las únicas escrituras son una fila de prueba por sistema a través de la pantalla, con el prefijo `ZZ_` y solo con tu aprobación.

| Paso | Qué pides | Qué ejecuta | Qué devuelve |
|---|---|---|---|
| 1. Qué hace el módulo | «Dame los flujos y las historias de usuario de `crm/quotes`» | `code flows` | Pantallas y acciones con sus condiciones y permisos, y después endpoints, controladores y tablas, como evidencia; las historias se escriben a partir de eso y tú las confirmas |
| 2. Qué toca el código viejo | «¿Qué llama y escribe el módulo viejo de cotizaciones, y qué tablas no tiene el nuevo?» | `code legacy {path, module}` | Cada endpoint viejo con quién lo llama y sus tablas; las tablas que solo tiene un lado, separadas en «nombradas en otra parte del backend» y «no nombradas» |
| 3. Pantallas | «Compara la lista de cotizaciones de la pantalla vieja y la nueva» | `page shape {name, selector}` en cada una, `page parity {a, b}` | Solo los campos, botones y columnas que tiene un lado |
| 4. Peticiones | «¿La búsqueda nueva envía y recibe lo mismo que la vieja?» | la misma acción en las dos, `page parity {req_a, req_b, map}` | El estado y los nombres de campo que solo envía o recibe un lado, nunca valores |
| 5. Qué probar | «Lista los casos de prueba de este módulo» | `code cases` | Una tabla: camino feliz, permiso, campo obligatorio, estado, error HTTP y lista vacía, con lo que ejecuta cada uno |
| 6. Una escritura en los dos | «Crea una cotización `ZZ_` en cada sistema y compara lo que se guardó» | `code snap`, `page fill` y `click`, `code snap_diff`, `code rows`, `query` | Las filas que escribió cada sistema, sus diferencias y las filas hijas por conteo |
| 7. Pruebas unitarias | «Planifica las pruebas unitarias que fijan el comportamiento viejo» | `code help {topic: "migration"}`, paso 7 | Cómo escribir cada diferencia confirmada como prueba unitaria, en la capa dueña de la regla |

La cadena completa, paso a paso, es `code help {topic: "migration"}`; el procedimiento para un módulo entero es [docs/module-testing.md](docs/module-testing.md) (en inglés). Una corrida real sobre un módulo de cotizaciones encontró, entre otras cosas, que 21 de 22 tablas «solo en el sistema viejo» en realidad las usaba el backend nuevo y que la pantalla vieja guardaba un número que la nueva ya había tomado (`Duplicate entry … 1062`). `CHANGELOG.md` (en inglés) tiene el detalle de cada corrida y lo que no se comprobó.

## Dónde pedir ayuda

- **Una duda o un fallo:** abre un [issue](https://github.com/partprogramming09/webscope/issues). Di la versión (`webscope` la imprime al arrancar), qué le pediste al agente, qué esperabas y qué pasó; un fallo se lee `Failed: <qué pasó>. Next: <qué hacer>`, pega esa línea.
- **Una vulnerabilidad:** no abras un issue; sigue [SECURITY.md](SECURITY.md) (en inglés).
- **Cómo usarlo:** [docs/usage.md](docs/usage.md) es el contrato de cada herramienta y el orden en que se usan; [docs/patterns.md](docs/patterns.md) lista los hallazgos al probar pantallas y cómo reconocer cada uno. Ambos, en inglés.

## Documentación

| Documento | Qué tiene |
|---|---|
| [docs/usage.md](docs/usage.md) | El contrato de cada herramienta y cómo usar las cuatro juntas. |
| [docs/reference.md](docs/reference.md) | Cada variable y argumento, lo medido en una aplicación real, los límites conocidos y lo que no está en esta versión. |
| [docs/install.md](docs/install.md) | Instalar en Claude Code (con plugin o sin él) y en otros agentes. |
| [docs/module-testing.md](docs/module-testing.md) | El procedimiento para probar un módulo como usuario y como atacante. |
| [CHANGELOG.md](CHANGELOG.md) | Una entrada por versión: qué cambió, qué se vio, cómo se comprobó y qué no. |

La documentación de `docs/` está en inglés.

## Cómo contribuir

Lo mantiene [partprogramming09](https://github.com/partprogramming09). Los cambios son bienvenidos: [CONTRIBUTING.md](CONTRIBUTING.md) (en inglés) dice cómo compilar, probar y cambiar el servidor (`go vet ./...` y `go test -race -count=1 ./...` es lo que corre la CI en Linux y macOS). Cada versión es un commit con su entrada del changelog.

## Licencia

[MIT](LICENSE).
