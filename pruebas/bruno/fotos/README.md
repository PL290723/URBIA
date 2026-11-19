# Fotos de prueba

Las pruebas que mandan fotos reales al modelo de IA las toman de esta carpeta. Las fotos **no
se suben al repositorio** (`.gitignore`): cada integrante pone las suyas con estos nombres.

| Archivo | Qué debe mostrar |
|---|---|
| `bache.jpeg` | Un bache claro, de cerca |
| `basura_dos.jpeg` | Basura acumulada en la vía pública |
| `basura_tres.jpeg` | Otra foto de basura acumulada |
| `limpia.jpg` | Una calle o plaza sin baches ni basura |

Reglas:

- Tomadas por el equipo: sin marcas de agua ni imágenes de internet o de medios.
- Sin personas reconocibles ni placas legibles.
- JPEG, de 200 a 8000 px por lado y menos de 10 MB.

Las pruebas que esperan una categoría concreta (por ejemplo, 09.4 espera que la IA sugiera
`bache`) pueden fallar con otra foto si el modelo no la reconoce; en ese caso, usar una foto más
clara del mismo problema.
