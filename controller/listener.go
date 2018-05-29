package controller

import (
	"net"
	"github.com/nextmetaphor/tcp-proxy-pool/application"
	"io"
	"crypto/tls"
	"github.com/nextmetaphor/tcp-proxy-pool/log"
)

const (
	logSecureServerStarting           = "Server starting on address [%s] and port [%s] with a secure configuration: cert[%s] key[%s]"
	logErrorCreatingListener          = "Error creating customTLSListener"
	logErrorAcceptingConnection       = "Error accepting connection"
	logErrorCopying                   = "Error copying"
	logErrorClosing                   = "Error closing"
	logErrorLoadingCertificates       = "Error loading certificates"
	logErrorServerConnNotTCP          = "Error: server connection not TCP"
	logErrorClientConnNotTCP          = "Error: client connection not TCP"
	logErrorAssigningContainer        = "Error: cannot assign container"
	logErrorProxyingConnection        = "Error proxying connection"
	logErrorCreatingMonitorConnection = "Error creating monitoring connection"
)

func (ctx *Context) StartListener() bool {
	ctx.InitialiseContainerPool(ECSContainerManager{})

	monitorClient := ctx.CreateMonitor()
	if monitorClient != nil {
		defer (*monitorClient).Close()
	}

	ctx.Logger.Infof(logSecureServerStarting,
		*(*ctx.Flags)[application.HostNameFlag].FlagValue,
		*(*ctx.Flags)[application.PortNameFlag].FlagValue,
		*(*ctx.Flags)[application.CertFileFlag].FlagValue,
		*(*ctx.Flags)[application.KeyFileFlag].FlagValue)

	tcpProtocol := *(*ctx.Flags)[application.TransportProtocolFlag].FlagValue
	tcpIP := *(*ctx.Flags)[application.HostNameFlag].FlagValue
	//tcpPort := *(*ctx.Flags)[application.PortNameFlag].FlagValue

	cert, err := tls.LoadX509KeyPair(*(*ctx.Flags)[application.CertFileFlag].FlagValue, *(*ctx.Flags)[application.KeyFileFlag].FlagValue)
	if err != nil {
		log.LogError(logErrorLoadingCertificates, err, ctx.Logger)
		return false
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, listenErr := Listen(tcpProtocol, tcpIP+":28443", tlsConfig)
	if listener != nil {
		defer listener.Close()
	}

	if listenErr != nil {
		log.LogError(logErrorCreatingListener, err, ctx.Logger)
		return false
	}

	ctx.handleConnections(listener)

	return true
}

// handleConnections is called when the container pool has been initialised and the listener has been started.
// A separate goroutine is created to handle each Accept request on the listener.
func (ctx *Context) handleConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.LogError(logErrorAcceptingConnection, err, ctx.Logger)
			return
		}

		go ctx.clientConnect(conn)
	}
}

// clientConnect is called in a separate goroutine for every successful Accept request on the server listener.
func (ctx *Context) clientConnect(serverConn net.Conn) {
	c, err := ctx.AssociateClientWithContainer(serverConn)
	if err != nil {
		ctx.Logger.Warn(logErrorAssigningContainer, err)
		// TODO - check for errors
		serverConn.Close()
		return
	}

	if err := ctx.ConnectClientToContainer(c); err != nil {
		log.LogError(logErrorProxyingConnection, err, ctx.Logger)
		return
	}

	ctx.proxy(c)
}

func (ctx *Context) proxy(c *Container) {
	server := c.ConnectionFromClient
	client := c.ConnectionToContainer

	clientClosedChannel := make(chan struct{}, 1)
	serverClosedChannel := make(chan struct{}, 1)

	go ctx.connectionCopy(false, server, client, clientClosedChannel)
	go ctx.connectionCopy(true, client, server, serverClosedChannel)

	var waitChannel chan struct{}
	select {
	case <-clientClosedChannel:
		if customTCPConn, customTCPConnErr := server.(*customTCPConn); customTCPConnErr {
			if tcpConn, tcpConnErr := customTCPConn.InnerConn.(*net.TCPConn); tcpConnErr {
				tcpConn.SetLinger(0)
				tcpConn.CloseRead()
			} else {
				ctx.Logger.Warn(logErrorServerConnNotTCP)
				customTCPConn.InnerConn.Close()
			}
		} else {
			ctx.Logger.Warn(logErrorServerConnNotTCP)
			server.Close()
		}
		waitChannel = serverClosedChannel

	case <-serverClosedChannel:
		if tcpConn, tcpConnErr := client.(*net.TCPConn); tcpConnErr {
			tcpConn.CloseRead()
		} else {
			ctx.Logger.Warn(logErrorClientConnNotTCP)
		}
		waitChannel = clientClosedChannel
	}

	<-waitChannel

	ctx.DissociateClientWithContainer(c)
}

func (ctx *Context) connectionCopy(srcIsServer bool, dst, src net.Conn, sourceClosedChannel chan struct{}) {
	_, err := io.Copy(dst, src);
	if err != nil {
		log.LogError(logErrorCopying, err, ctx.Logger)
	}

	if err := src.Close(); err != nil {
		log.LogError(logErrorClosing, err, ctx.Logger)
	}

	sourceClosedChannel <- struct{}{}
}
