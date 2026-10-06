//go:build postgres

package store_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func TestTransactReadsCommitted(t *testing.T) {
	ctx := context.Background()
	admin := storetest.New(t)
	var name string
	if err := admin.QueryRow(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	alter := fmt.Sprintf(`ALTER DATABASE %s SET default_transaction_isolation = 'repeatable read'`, pgx.Identifier{name}.Sanitize())
	if _, err := admin.Exec(ctx, alter); err != nil {
		t.Fatal(err)
	}
	// A fresh pool opens fresh connections, which take the new default.
	pool, err := store.Open(ctx, admin.Config().ConnString(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var plain string
	if err := pool.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&plain); err != nil || plain != "repeatable read" {
		t.Fatalf("default isolation = %q (%v)", plain, err)
	}
	var level string
	err = store.Transact(ctx, pool, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&level)
	})
	if err != nil {
		t.Fatal(err)
	}
	if level != "read committed" {
		t.Fatalf("isolation = %q", level)
	}
}

// blackHole forwards a TCP stream to a server until blocked is set; from
// then on it drops what the client sends, so the server never answers. The
// test's cleanup closes every stream.
type blackHole struct {
	listener net.Listener
	blocked  atomic.Bool
	mu       sync.Mutex
	conns    []net.Conn
}

func newBlackHole(t *testing.T, upstream string) *blackHole {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	b := &blackHole{listener: l}
	var streams sync.WaitGroup
	t.Cleanup(func() {
		b.close()
		streams.Wait()
	})
	streams.Go(func() {
		for {
			client, err := l.Accept()
			if err != nil {
				return
			}
			server, err := net.Dial("tcp", upstream)
			if err != nil {
				_ = client.Close()
				continue
			}
			b.mu.Lock()
			b.conns = append(b.conns, client, server)
			b.mu.Unlock()
			streams.Go(func() { _, _ = io.Copy(client, server) })
			streams.Go(func() {
				buf := make([]byte, 32*1024)
				for {
					n, err := client.Read(buf)
					if n > 0 && !b.blocked.Load() {
						if _, werr := server.Write(buf[:n]); werr != nil {
							return
						}
					}
					if err != nil {
						return
					}
				}
			})
		}
	})
	return b
}

// close stops the listener and closes every stream.
func (b *blackHole) close() {
	_ = b.listener.Close()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.conns {
		_ = c.Close()
	}
}

// A database that stops answering in the middle of a transaction does not
// hold Transact forever: the rollback has its own bound.
func TestTransactBoundsTheRollback(t *testing.T) {
	ctx := context.Background()
	admin := storetest.New(t)
	cfg := admin.Config().Copy()
	upstream := net.JoinHostPort(cfg.ConnConfig.Host, strconv.Itoa(int(cfg.ConnConfig.Port)))
	hole := newBlackHole(t, upstream)
	host, port, err := net.SplitHostPort(hole.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Fallbacks = host, uint16(p), nil
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	failure := errors.New("the work failed")
	start := time.Now()
	done := make(chan error, 1)
	go func() {
		done <- store.Transact(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SELECT 1`); err != nil {
				return err
			}
			hole.blocked.Store(true)
			return failure
		})
	}()
	select {
	case err := <-done:
		if !errors.Is(err, failure) {
			t.Fatalf("err = %v", err)
		}
		t.Logf("Transact returned after %v", time.Since(start).Round(time.Second))
		// A connection closed by its peer closes at once.
		hole.close()
		pool.Close()
	case <-time.After(15 * time.Second):
		// The pool is left open: closing it would wait for the
		// connection the rollback still holds.
		t.Fatal("Transact did not return 15 seconds after the database stopped answering")
	}
}
