package pkg

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"syscall"
	"time"

	"ChatIM/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Run(r *gin.Engine, srvName string, addr string, stop func()) {

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		logger.Info("Server starting", zap.String("name", srvName), zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server listen failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	logger.Info("Shutting down server", zap.String("name", srvName))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if stop != nil {
		stop()
	}
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.String("name", srvName), zap.Error(err))
	}
	select {
	case <-ctx.Done():
		logger.Warn("Shutdown timeout")
	}
	logger.Info("Server stopped successfully", zap.String("name", srvName))

}
