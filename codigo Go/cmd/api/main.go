package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"orders-go/internal/config"
	"orders-go/internal/order"
	"orders-go/internal/platform/db"
	"orders-go/internal/platform/logger"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fallo al ejecutar el servicio", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log, err := logger.New(cfg)
	if err != nil {
		return err
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := db.NewPool(startupCtx, cfg)
	cancelStartup()
	if err != nil {
		return err
	}
	defer pool.Close()

	repository := order.NewPostgresRepository(pool)
	usecase := order.NewUseCase(repository)
	apiHandler := order.NewHandler(usecase, log, pool.Ping)

	mux := http.NewServeMux()
	apiHandler.Register(mux)
	handler := requestIDMiddleware(
		log,
		loggingMiddleware(log, recoveryMiddleware(log, requestTimeoutMiddleware(cfg.RequestTimeout, mux))),
	)

	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.APIHost, strconv.Itoa(cfg.APIPort)),
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	serverErrors := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}
		serverErrors <- nil
	}()

	var serverErr error
	select {
	case <-signalCtx.Done():
		log.Info("senal de apagado recibida")
	case serverErr = <-serverErrors:
		if serverErr != nil {
			log.Error("servidor HTTP detenido por error", "error", serverErr)
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("servidor detenido correctamente")
	return serverErr
}

func requestIDMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rawID [16]byte
		if _, err := rand.Read(rawID[:]); err != nil {
			log.Error("no se pudo generar el identificador de solicitud", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("{\"error\":\"Error interno\",\"code\":\"internal_error\"}\n"))
			return
		}
		requestID := hex.EncodeToString(rawID[:])
		w.Header().Set("X-Request-ID", requestID)
		ctx := order.WithRequestID(r.Context(), requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.written {
		return
	}
	w.status = status
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusRecorder) Written() bool {
	return w.written
}

func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w}
		started := time.Now()
		defer func() {
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Info("solicitud HTTP",
				"request_id", order.RequestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration", time.Since(started).String(),
			)
		}()
		next.ServeHTTP(recorder, r)
	})
}

func requestTimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func recoveryMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recuperado",
					"request_id", order.RequestIDFromContext(r.Context()),
					"panic", recovered,
				)
				if recorder, ok := w.(interface{ Written() bool }); !ok || !recorder.Written() {
					writeInternalError(w)
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func writeInternalError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte("{\"error\":\"Error interno\",\"code\":\"internal_error\"}\n"))
}
