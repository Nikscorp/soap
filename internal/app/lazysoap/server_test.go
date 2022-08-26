package lazysoap

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	address := "127.0.0.1:50042"
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	srv := Server{address, tvMetaClientMock}
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

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "pong", string(body))

	cancel()
	<-done
	require.ErrorIs(t, err, http.ErrServerClosed)
}
