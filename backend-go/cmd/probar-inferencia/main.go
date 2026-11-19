// probar-inferencia llama al servicio Python por gRPC con el contrato
// urbia.inferencia.v1 (HU 05.4). Sirve para comprobar a mano que Go y
// Python se comunican, sin levantar la API ni la base de datos.
//
// Uso (desde backend-go/, con el servidor Python corriendo):
//
//	go run ./cmd/probar-inferencia -info
//	go run ./cmd/probar-inferencia -ruta originales/2026/11/<26 caracteres>.jpg
//	go run ./cmd/probar-inferencia -ruta ../../etc/passwd        (error controlado)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
)

func main() {
	addr := flag.String("addr", valorODefecto(os.Getenv("INFERENCE_GRPC_ADDR"), "localhost:50051"), "host:puerto del servicio Python")
	timeout := flag.Duration("timeout", 10*time.Second, "tiempo máximo por llamada")
	info := flag.Bool("info", false, "solo mostrar el modelo cargado")
	ruta := flag.String("ruta", "", "ruta relativa a STORAGE_DIR (archivo.ruta)")
	reporte := flag.Int64("reporte", 1, "id_reporte (solo para los logs)")
	archivo := flag.Int64("archivo", 1, "id_archivo (solo para los logs)")
	flag.Parse()

	c, err := clients.NuevoClienteDeteccion(*addr, *timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer c.Cerrar()
	ctx := context.Background()

	if *info || *ruta == "" {
		i, err := c.Info(ctx)
		salir(i, err)
	}

	inicio := time.Now()
	d, err := c.Detectar(ctx, clients.SolicitudDeteccion{
		IDSolicitud: fmt.Sprintf("cli-%d", time.Now().UnixNano()),
		IDReporte:   *reporte, IDArchivo: *archivo, Ruta: *ruta,
	})
	if err == nil {
		fmt.Fprintf(os.Stderr, "ida y vuelta: %d ms\n", time.Since(inicio).Milliseconds())
	}
	salir(d, err)
}

func salir(v any, err error) {
	if err != nil {
		var e *clients.ErrorInferencia
		if errors.As(err, &e) {
			imprimir(map[string]any{"codigo": e.Codigo, "grpc": e.GRPC.String(),
				"mensaje": e.Mensaje, "reintentable": e.Reintentable()})
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
	imprimir(v)
	os.Exit(0)
}

func imprimir(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func valorODefecto(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
