package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// maxIPsRegistradas evita que el mapa crezca sin fin si alguien rota IPs.
const maxIPsRegistradas = 10_000

// LimiteIntentos frena la fuerza bruta en el login (HU 09.1).
//
// Cuenta las respuestas 401 por IP. Al llegar a max dentro de la ventana, TODO
// intento de esa IP responde 429 (aunque traiga la contraseña correcta) hasta
// que la ventana termine; así no sirve seguir probando. Un login correcto no
// reinicia el contador. La IP sale de c.ClientIP(), y como el router no confía
// en proxies (SetTrustedProxies(nil)), X-Forwarded-For no sirve para burlarlo.
//
// Es memoria del proceso: se reinicia al reiniciar la API. Suficiente para una
// demostración en red local con una sola instancia.
func LimiteIntentos(max int, ventana time.Duration) gin.HandlerFunc {
	l := &limitadorIntentos{max: max, ventana: ventana, porIP: map[string]*registroIntentos{}}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if espera, bloqueado := l.bloqueado(ip); bloqueado {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
			response.Error(c, http.StatusTooManyRequests, response.CodeTooManyRequests,
				"Demasiados intentos fallidos. Espera unos minutos antes de volver a intentar")
			return
		}
		c.Next()
		if c.Writer.Status() == http.StatusUnauthorized {
			l.registrarFallo(ip)
		}
	}
}

type limitadorIntentos struct {
	mu      sync.Mutex
	max     int
	ventana time.Duration
	porIP   map[string]*registroIntentos
}

type registroIntentos struct {
	fallidos int
	inicio   time.Time
}

// bloqueado indica si la IP ya agotó sus intentos y cuánto falta para que se libere.
func (l *limitadorIntentos) bloqueado(ip string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	r, ok := l.porIP[ip]
	if !ok {
		return 0, false
	}
	restante := l.ventana - time.Since(r.inicio)
	if restante <= 0 {
		delete(l.porIP, ip)
		return 0, false
	}
	return restante, r.fallidos >= l.max
}

func (l *limitadorIntentos) registrarFallo(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ahora := time.Now()
	r, ok := l.porIP[ip]
	if ok && ahora.Sub(r.inicio) < l.ventana {
		r.fallidos++
		return
	}
	if len(l.porIP) >= maxIPsRegistradas {
		l.purgar(ahora)
	}
	l.porIP[ip] = &registroIntentos{fallidos: 1, inicio: ahora}
}

// purgar borra las ventanas vencidas. Se llama con el mutex tomado.
func (l *limitadorIntentos) purgar(ahora time.Time) {
	for ip, r := range l.porIP {
		if ahora.Sub(r.inicio) >= l.ventana {
			delete(l.porIP, ip)
		}
	}
}
