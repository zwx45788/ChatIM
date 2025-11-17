package pkg

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func Run(r *gin.Engine, srvName string, addr string, stop func()) {

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("%s running in %s\n", srvName, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Printf("Shutting down project %s...\n", srvName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if stop != nil {
		stop()
	}
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("%s forced to shutdown: %v", srvName, err)
	}
	select {
	case <-ctx.Done():
		log.Print("wait time out...")
	}
	log.Printf("%s stop success...\n", srvName)

}
