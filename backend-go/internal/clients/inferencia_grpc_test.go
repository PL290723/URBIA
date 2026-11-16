package clients

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/PL290723/URBIA/backend-go/internal/gen/inferenciav1"
)

// servidorFalso implementa el contrato como lo hace Python, en memoria.
type servidorFalso struct {
	pb.UnimplementedInferenciaServiceServer
	ultima    *pb.DetectarRequest
	respuesta *pb.DetectarResponse
	codigo    string // si no es "", responde con error y este trailer
	grpcCode  codes.Code
	espera    time.Duration
}

func (s *servidorFalso) Detectar(ctx context.Context, r *pb.DetectarRequest) (*pb.DetectarResponse, error) {
	s.ultima = r
	select {
	case <-time.After(s.espera):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if s.codigo != "" {
		_ = grpc.SetTrailer(ctx, metadata.Pairs(TrailerCodigo, s.codigo))
		return nil, status.Error(s.grpcCode, "detalle desde Python")
	}
	return s.respuesta, nil
}

func (s *servidorFalso) ObtenerInfo(context.Context, *pb.ObtenerInfoRequest) (*pb.ObtenerInfoResponse, error) {
	return &pb.ObtenerInfoResponse{
		VersionContrato: "1.0.0", Clases: []string{"bache", "basura"}, Imgsz: 640, Dispositivo: "cpu",
		Modelo: &pb.ModeloUsado{Nombre: "urbia-yolo26s", Version: "1.0.0", UmbralConfianza: 0.25},
	}, nil
}

func respuestaDetectada() *pb.DetectarResponse {
	return &pb.DetectarResponse{
		IdSolicitud: "req-1", Resultado: pb.Resultado_RESULTADO_DETECTADO,
		ClaseDetectada: "basura", Confianza: 0.9123,
		Cajas:  []*pb.Caja{{Clase: "basura", Confianza: 0.9123, X1: 211.3, Y1: 20, X2: 110, Y2: 220}},
		Modelo: &pb.ModeloUsado{Nombre: "urbia-yolo26s", Version: "1.0.0", UmbralConfianza: 0.25},
		Ancho:  2560, Alto: 1920, Milisegundos: 180.5,
	}
}

func nuevoPar(t *testing.T, timeout time.Duration) (*servidorFalso, *ClienteDeteccion) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	falso := &servidorFalso{respuesta: respuestaDetectada()}
	pb.RegisterInferenciaServiceServer(srv, falso)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	c, err := NuevoClienteDeteccion("passthrough:///bufnet", timeout,
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Cerrar() })
	return falso, c
}

var solicitud = SolicitudDeteccion{
	IDSolicitud: "req-1", IDReporte: 7, IDArchivo: 42,
	Ruta: "originales/2026/11/abcdefghijklmnopqrstuvwxyz.jpg", MaxCajas: 5,
}

func TestDetectar_Detectado(t *testing.T) {
	falso, c := nuevoPar(t, 2*time.Second)
	d, err := c.Detectar(context.Background(), solicitud)
	if err != nil {
		t.Fatal(err)
	}
	r := falso.ultima
	if r.GetIdReporte() != 7 || r.GetIdArchivo() != 42 || r.GetRutaArchivo() != solicitud.Ruta ||
		r.GetIdSolicitud() != "req-1" || r.GetOpciones().GetMaxCajas() != 5 {
		t.Fatalf("la solicitud no llegó completa: %v", r)
	}
	if d.Resultado != ResultadoDetectado || d.ClaseDetectada == nil || *d.ClaseDetectada != "basura" {
		t.Fatalf("detección inesperada: %+v", d)
	}
	if *d.Confianza != 0.9123 || d.UmbralConfianza != 0.25 {
		t.Fatalf("confianza/umbral sin redondear a 4 decimales: %v %v", *d.Confianza, d.UmbralConfianza)
	}
	if d.Cajas[0].X1 != 211.3 || d.Milisegundos != 180.5 {
		t.Fatalf("píxeles/milisegundos sin redondear a 1 decimal: %v %v", d.Cajas[0].X1, d.Milisegundos)
	}
	if d.Modelo != "urbia-yolo26s" || d.VersionModelo != "1.0.0" || d.Ancho != 2560 || len(d.Cajas) != 1 {
		t.Fatalf("detección incompleta: %+v", d)
	}
}

func TestDetectar_NoDetectadoEsNULL(t *testing.T) {
	falso, c := nuevoPar(t, 2*time.Second)
	falso.respuesta = &pb.DetectarResponse{Resultado: pb.Resultado_RESULTADO_NO_DETECTADO,
		Modelo: &pb.ModeloUsado{Nombre: "urbia-yolo26s", Version: "1.0.0"}}
	d, err := c.Detectar(context.Background(), solicitud)
	if err != nil {
		t.Fatal(err)
	}
	if d.Resultado != ResultadoNoDetectado || d.ClaseDetectada != nil || d.Confianza != nil || len(d.Cajas) != 0 {
		t.Fatalf("no_detectado debe dejar clase y confianza en nil: %+v", d)
	}
}

func TestDetectar_ErroresDePython(t *testing.T) {
	casos := []struct {
		codigo       string
		grpcCode     codes.Code
		reintentable bool
	}{
		{"IMAGEN_INVALIDA", codes.InvalidArgument, false},
		{"RUTA_INVALIDA", codes.InvalidArgument, false},
		{"IMAGEN_NO_ENCONTRADA", codes.NotFound, false},
		{"FALLO_INFERENCIA", codes.Internal, true},
		{"MODELO_NO_DISPONIBLE", codes.Unavailable, true},
	}
	for _, caso := range casos {
		t.Run(caso.codigo, func(t *testing.T) {
			falso, c := nuevoPar(t, 2*time.Second)
			falso.codigo, falso.grpcCode = caso.codigo, caso.grpcCode
			_, err := c.Detectar(context.Background(), solicitud)
			var e *ErrorInferencia
			if !errors.As(err, &e) {
				t.Fatalf("se esperaba *ErrorInferencia, llegó %T: %v", err, err)
			}
			if e.Codigo != caso.codigo || e.GRPC != caso.grpcCode || e.Mensaje != "detalle desde Python" {
				t.Fatalf("error mal traducido: %+v", e)
			}
			if e.Reintentable() != caso.reintentable {
				t.Fatalf("Reintentable() = %v, se esperaba %v", e.Reintentable(), caso.reintentable)
			}
		})
	}
}

func TestDetectar_Timeout(t *testing.T) {
	falso, c := nuevoPar(t, 100*time.Millisecond)
	falso.espera = time.Second
	inicio := time.Now()
	_, err := c.Detectar(context.Background(), solicitud)
	var e *ErrorInferencia
	if !errors.As(err, &e) || e.Codigo != CodigoTiempoAgotado || e.GRPC != codes.DeadlineExceeded {
		t.Fatalf("se esperaba TIEMPO_AGOTADO, llegó %v", err)
	}
	if time.Since(inicio) > 500*time.Millisecond {
		t.Fatal("el cliente no respetó su timeout")
	}
	if !e.Reintentable() {
		t.Fatal("un timeout debe ser reintentable")
	}
}

func TestDetectar_ContextoCancelado(t *testing.T) {
	falso, c := nuevoPar(t, 2*time.Second)
	falso.espera = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	_, err := c.Detectar(ctx, solicitud)
	var e *ErrorInferencia
	if !errors.As(err, &e) || e.Codigo != CodigoCancelado {
		t.Fatalf("se esperaba CANCELADO, llegó %v", err)
	}
}

func TestDetectar_ServicioCaido(t *testing.T) {
	// Nadie escucha en el puerto 1. En Windows la conexión rechazada tarda ~2 s
	// (reintentos de TCP), así que el timeout debe ser mayor para recibir Unavailable.
	c, err := NuevoClienteDeteccion("127.0.0.1:1", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Cerrar()
	_, err = c.Detectar(context.Background(), solicitud)
	var e *ErrorInferencia
	if !errors.As(err, &e) || e.Codigo != CodigoServicioNoDisponible || !e.Reintentable() {
		t.Fatalf("se esperaba SERVICIO_NO_DISPONIBLE reintentable, llegó %v", err)
	}
}

func TestDetectar_ResultadoDesconocidoEsContratoInvalido(t *testing.T) {
	falso, c := nuevoPar(t, 2*time.Second)
	falso.respuesta = &pb.DetectarResponse{Resultado: pb.Resultado_RESULTADO_NO_ESPECIFICADO}
	_, err := c.Detectar(context.Background(), solicitud)
	var e *ErrorInferencia
	if !errors.As(err, &e) || e.Codigo != CodigoContratoInvalido {
		t.Fatalf("se esperaba CONTRATO_INVALIDO, llegó %v", err)
	}
}

func TestDetectar_DetectadoSinClaseEsContratoInvalido(t *testing.T) {
	falso, c := nuevoPar(t, 2*time.Second)
	falso.respuesta = &pb.DetectarResponse{Resultado: pb.Resultado_RESULTADO_DETECTADO}
	if _, err := c.Detectar(context.Background(), solicitud); err == nil {
		t.Fatal("se esperaba error por DETECTADO sin clase")
	}
}

func TestInfo(t *testing.T) {
	_, c := nuevoPar(t, 2*time.Second)
	i, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if i.VersionContrato != VersionContrato || i.Modelo != "urbia-yolo26s" || len(i.Clases) != 2 || i.UmbralConfianza != 0.25 {
		t.Fatalf("info inesperada: %+v", i)
	}
}

func TestNuevoClienteDeteccion_Validaciones(t *testing.T) {
	if _, err := NuevoClienteDeteccion("", time.Second); err == nil {
		t.Fatal("dirección vacía debería fallar")
	}
	if _, err := NuevoClienteDeteccion("localhost:50051", 0); err == nil {
		t.Fatal("timeout 0 debería fallar")
	}
}
