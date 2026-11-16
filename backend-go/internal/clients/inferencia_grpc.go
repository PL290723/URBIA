package clients

// Cliente del contrato gRPC urbia.inferencia.v1 (HU 05.4).
// Contrato: proto/urbia/inferencia/v1/inferencia.proto
// Documentación: docs/vision/contrato-grpc.md

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/PL290723/URBIA/backend-go/internal/gen/inferenciav1"
)

// VersionContrato es la versión del .proto con la que se compiló este cliente.
const VersionContrato = "1.0.0"

// TrailerCodigo es el metadato donde Python envía el código de error de URBIA.
const TrailerCodigo = "urbia-codigo"

// Valores de Deteccion.Resultado; coinciden con ck de la columna deteccion.resultado.
const (
	ResultadoDetectado   = "detectado"
	ResultadoNoDetectado = "no_detectado"
)

// Códigos que genera el propio cliente (los demás vienen de Python).
const (
	CodigoServicioNoDisponible = "SERVICIO_NO_DISPONIBLE"
	CodigoTiempoAgotado        = "TIEMPO_AGOTADO"
	CodigoCancelado            = "CANCELADO"
	CodigoContratoInvalido     = "CONTRATO_INVALIDO"
	CodigoDesconocido          = "ERROR_DESCONOCIDO"
)

// ErrorInferencia es el error de cualquier llamada al servicio de inferencia.
type ErrorInferencia struct {
	Codigo  string     // código de URBIA: IMAGEN_INVALIDA, TIEMPO_AGOTADO, …
	GRPC    codes.Code // código de estado gRPC
	Mensaje string
}

func (e *ErrorInferencia) Error() string {
	return fmt.Sprintf("inferencia %s (%s): %s", e.Codigo, e.GRPC, e.Mensaje)
}

// Reintentable indica si tiene sentido volver a intentar más tarde
// (el servicio no estaba disponible, tardó demasiado o falló el modelo).
// Una imagen inválida o inexistente nunca se arregla reintentando.
func (e *ErrorInferencia) Reintentable() bool {
	switch e.GRPC {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Internal:
		return true
	}
	return false
}

// SolicitudDeteccion son los datos que Go conoce de la foto (tablas reporte y archivo).
type SolicitudDeteccion struct {
	IDSolicitud string // X-Request-ID, para seguir la petición en los logs de ambos servicios
	IDReporte   int64
	IDArchivo   int64
	Ruta        string // archivo.ruta, relativa a STORAGE_DIR
	OmitirCajas bool
	MaxCajas    uint32 // 0 = todas
}

type Caja struct {
	Clase     string  `json:"clase"`
	Confianza float64 `json:"confianza"`
	X1        float64 `json:"x1"`
	Y1        float64 `json:"y1"`
	X2        float64 `json:"x2"`
	Y2        float64 `json:"y2"`
}

// Deteccion usa los mismos nombres y tipos que la tabla deteccion:
// ClaseDetectada y Confianza son nil (NULL) cuando no hubo detección.
type Deteccion struct {
	Resultado       string   `json:"resultado"`
	ClaseDetectada  *string  `json:"clase_detectada"`
	Confianza       *float64 `json:"confianza"`
	Modelo          string   `json:"modelo"`
	VersionModelo   string   `json:"version_modelo"`
	UmbralConfianza float64  `json:"umbral_confianza"`
	Cajas           []Caja   `json:"cajas"`
	Ancho           int      `json:"ancho"`
	Alto            int      `json:"alto"`
	Milisegundos    float64  `json:"milisegundos"`
}

type InfoServicio struct {
	VersionContrato string   `json:"version_contrato"`
	Modelo          string   `json:"modelo"`
	VersionModelo   string   `json:"version_modelo"`
	UmbralConfianza float64  `json:"umbral_confianza"`
	Clases          []string `json:"clases"`
	Imgsz           int      `json:"imgsz"`
	Dispositivo     string   `json:"dispositivo"`
	SHA256Pesos     string   `json:"sha256_pesos"`
	Peticiones      uint64   `json:"peticiones"`
	Errores         uint64   `json:"errores"`
}

// ClienteDeteccion habla con el servicio Python por gRPC.
// Es seguro para usarse desde varias goroutines; se crea una vez al iniciar.
type ClienteDeteccion struct {
	conn    *grpc.ClientConn
	api     pb.InferenciaServiceClient
	timeout time.Duration
}

// NuevoClienteDeteccion no se conecta todavía: la conexión se abre en la
// primera llamada y se reconecta sola si el servicio Python se reinicia.
func NuevoClienteDeteccion(direccion string, timeout time.Duration, opciones ...grpc.DialOption) (*ClienteDeteccion, error) {
	if direccion == "" {
		return nil, errors.New("dirección gRPC de inferencia vacía")
	}
	if timeout <= 0 {
		return nil, errors.New("el timeout de inferencia debe ser mayor que 0")
	}
	opts := append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, opciones...)
	conn, err := grpc.NewClient(direccion, opts...)
	if err != nil {
		return nil, fmt.Errorf("cliente gRPC de inferencia: %w", err)
	}
	return &ClienteDeteccion{conn: conn, api: pb.NewInferenciaServiceClient(conn), timeout: timeout}, nil
}

func (c *ClienteDeteccion) Cerrar() error { return c.conn.Close() }

// Detectar pide la detección de una foto. Cada llamada tiene su propio
// límite de tiempo (INFERENCE_GRPC_TIMEOUT), además del que traiga ctx.
func (c *ClienteDeteccion) Detectar(ctx context.Context, s SolicitudDeteccion) (*Deteccion, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var trailer metadata.MD
	resp, err := c.api.Detectar(ctx, &pb.DetectarRequest{
		IdSolicitud: s.IDSolicitud,
		IdReporte:   s.IDReporte,
		IdArchivo:   s.IDArchivo,
		RutaArchivo: s.Ruta,
		Opciones:    &pb.OpcionesDeteccion{OmitirCajas: s.OmitirCajas, MaxCajas: s.MaxCajas},
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, traducirError(err, trailer)
	}
	return aDeteccion(resp)
}

// Info devuelve el modelo cargado y los contadores del servicio Python.
func (c *ClienteDeteccion) Info(ctx context.Context) (*InfoServicio, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var trailer metadata.MD
	r, err := c.api.ObtenerInfo(ctx, &pb.ObtenerInfoRequest{}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, traducirError(err, trailer)
	}
	modelo := r.GetModelo()
	return &InfoServicio{
		VersionContrato: r.GetVersionContrato(),
		Modelo:          modelo.GetNombre(),
		VersionModelo:   modelo.GetVersion(),
		UmbralConfianza: redondear(modelo.GetUmbralConfianza()),
		Clases:          r.GetClases(),
		Imgsz:           int(r.GetImgsz()),
		Dispositivo:     r.GetDispositivo(),
		SHA256Pesos:     r.GetSha256Pesos(),
		Peticiones:      r.GetPeticiones(),
		Errores:         r.GetErrores(),
	}, nil
}

func aDeteccion(r *pb.DetectarResponse) (*Deteccion, error) {
	d := &Deteccion{
		Modelo:          r.GetModelo().GetNombre(),
		VersionModelo:   r.GetModelo().GetVersion(),
		UmbralConfianza: redondear(r.GetModelo().GetUmbralConfianza()),
		Ancho:           int(r.GetAncho()),
		Alto:            int(r.GetAlto()),
		Milisegundos:    redondear1(r.GetMilisegundos()),
		Cajas:           make([]Caja, 0, len(r.GetCajas())),
	}
	switch r.GetResultado() {
	case pb.Resultado_RESULTADO_DETECTADO:
		if r.GetClaseDetectada() == "" {
			return nil, contratoInvalido("resultado DETECTADO sin clase_detectada")
		}
		clase, conf := r.GetClaseDetectada(), redondear(r.GetConfianza())
		d.Resultado, d.ClaseDetectada, d.Confianza = ResultadoDetectado, &clase, &conf
	case pb.Resultado_RESULTADO_NO_DETECTADO:
		d.Resultado = ResultadoNoDetectado
	default:
		// Python envió un valor que este cliente no conoce: versiones distintas del .proto
		return nil, contratoInvalido(fmt.Sprintf("resultado desconocido %v", r.GetResultado()))
	}
	for _, c := range r.GetCajas() {
		d.Cajas = append(d.Cajas, Caja{
			Clase: c.GetClase(), Confianza: redondear(c.GetConfianza()),
			X1: redondear1(c.GetX1()), Y1: redondear1(c.GetY1()), X2: redondear1(c.GetX2()), Y2: redondear1(c.GetY2()),
		})
	}
	return d, nil
}

func traducirError(err error, trailer metadata.MD) *ErrorInferencia {
	st, _ := status.FromError(err)
	e := &ErrorInferencia{GRPC: st.Code(), Mensaje: st.Message()}
	if v := trailer.Get(TrailerCodigo); len(v) > 0 && v[0] != "" {
		e.Codigo = v[0]
		return e
	}
	// Sin trailer: el error no salió del servicio Python (red, timeout del cliente, …)
	switch st.Code() {
	case codes.Unavailable:
		e.Codigo = CodigoServicioNoDisponible
	case codes.DeadlineExceeded:
		e.Codigo = CodigoTiempoAgotado
	case codes.Canceled:
		e.Codigo = CodigoCancelado
	case codes.Unimplemented:
		e.Codigo = CodigoContratoInvalido
	default:
		e.Codigo = CodigoDesconocido
	}
	return e
}

func contratoInvalido(msg string) *ErrorInferencia {
	return &ErrorInferencia{Codigo: CodigoContratoInvalido, GRPC: codes.Unknown, Mensaje: msg}
}

// protobuf usa float32; se redondea para no arrastrar ruido como 0.91229999.
// Confianza y umbral: 4 decimales (NUMERIC(5,4) en la BD).
func redondear(v float32) float64 {
	return math.Round(float64(v)*10000) / 10000
}

// Píxeles y milisegundos: 1 decimal.
func redondear1(v float32) float64 {
	return math.Round(float64(v)*10) / 10
}
