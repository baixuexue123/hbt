package hbt

import (
	"bufio"
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

// onceCloseListener wraps a net.Listener, protecting it from
// multiple Close calls.
type onceCloseListener struct {
	net.Listener
	once     sync.Once
	closeErr error
}

func (oc *onceCloseListener) Close() error {
	oc.once.Do(oc.close)
	return oc.closeErr
}

func (oc *onceCloseListener) close() { oc.closeErr = oc.Listener.Close() }

func handleMessage(msg string, c net.Conn) {
	if msg == "ping" {
		c.Write([]byte("pong\n"))
	} else {
		c.Write([]byte("unknown\n"))
	}
}

type TCPServer struct {
	Addr       string
	mu         sync.Mutex
	listener   *net.Listener
	activeConn map[*net.Conn]struct{}
	doneChan   chan struct{}
	onShutdown []func()

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func (s *TCPServer) getDoneChan() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDoneChanLocked()
}

func (s *TCPServer) getDoneChanLocked() chan struct{} {
	if s.doneChan == nil {
		s.doneChan = make(chan struct{})
	}
	return s.doneChan
}

func (s *TCPServer) closeDoneChanLocked() {
	ch := s.getDoneChanLocked()
	select {
	case <-ch:
		// Already closed. Don't close again.
	default:
		// Safe to close here. We're the only closer, guarded
		// by s.mu.
		close(ch)
	}
}

func (s *TCPServer) ListenAndServe() error {
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	l = tcpKeepAliveListener{l.(*net.TCPListener)}
	l = &onceCloseListener{Listener: l}
	defer l.Close()
	s.listener = &l

	return s.serve()
}

var ErrServerClosed = errors.New("server closed")

func (s *TCPServer) serve() error {
	ctx := context.Background()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := (*s.listener).Accept()
		if err != nil {
			select {
			case <-s.getDoneChan():
				return ErrServerClosed
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		go s.serveConn(ctx, conn)
	}
}

func (s *TCPServer) serveConn(ctx context.Context, c net.Conn) {
	peer := c.RemoteAddr().String()
	log.Println("new connection from", peer)
	s.addConn(&c)
	defer s.removeConn(&c)
	defer c.Close()
	scanner := bufio.NewScanner(c)
	for scanner.Scan() {
		msg := scanner.Text()
		if len(msg) == 0 {
			break
		}
		handleMessage(msg, c)
	}
	log.Println("connection:", peer, "lost")
}

// Close immediately closes net.Listener and all connections
func (s *TCPServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeDoneChanLocked()
	err := (*s.listener).Close()
	for c := range s.activeConn {
		e := (*c).Close()
		if e != nil && err == nil {
			err = e
		}
		delete(s.activeConn, c)
	}
	return err
}

var shutdownPollInterval = 500 * time.Millisecond

// gracefully shutdown
func (s *TCPServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	_ = (*s.listener).Close()
	s.closeDoneChanLocked()
	for _, f := range s.onShutdown {
		go f()
	}
	s.mu.Unlock()

	ticker := time.NewTicker(shutdownPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *TCPServer) RegisterOnShutdown(f func()) {
	s.mu.Lock()
	s.onShutdown = append(s.onShutdown, f)
	s.mu.Unlock()
}

func (s *TCPServer) addConn(conn *net.Conn) {
	s.mu.Lock()
	s.activeConn[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *TCPServer) removeConn(conn *net.Conn) {
	s.mu.Lock()
	delete(s.activeConn, conn)
	s.mu.Unlock()
}

func (s *TCPServer) ConnNum() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.activeConn)
}

func NewTCPServer(addr string) *TCPServer {
	var ts *TCPServer
	if addr == "" {
		addr = "localhost:8080"
	}
	ts.Addr = addr
	ts.activeConn = make(map[*net.Conn]struct{})
	return ts
}
