package lazysoap

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	address := "127.0.0.1:50042"
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	srv := Server{address, tvMetaClientMock, rest.NewMetrics(), nil}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	done := make(chan struct{})

	go func() {
		err = srv.Run(ctx)
		done <- struct{}{}
	}()
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + address + "/ping")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "pong", string(body))

	cancel()
	<-done
	require.ErrorIs(t, err, http.ErrServerClosed)
}
