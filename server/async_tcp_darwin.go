package server

import (
	"log"
	"net"
	"redis_internals/config"
	"redis_internals/core"
	"syscall"
)

func RunAsyncTCPServer() error {
	log.Println("starting an asychronous TCP server on", config.Host, config.Port)

	max_clients := 20000

	// Create Kqueue Event Objects to hold events
	var events []syscall.Kevent_t = make([]syscall.Kevent_t, max_clients)

	// Create a Socket
	serverFD, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(serverFD)

	// Set the Socket operate in a non-blocking mode
	if err = syscall.SetNonblock(serverFD, true); err != nil {
		return err
	}

	// Bind the IP and the port
	ip4 := net.ParseIP(config.Host)
	if err = syscall.Bind(serverFD, &syscall.SockaddrInet4{
		Port: config.Port,
		Addr: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]},
	}); err != nil {
		return err
	}

	// start listening
	if err = syscall.Listen(serverFD, max_clients); err != nil {
		return err
	}

	// AsyncIO starts here!!

	// creating KQUEUE instance
	kqueueFD, err := syscall.Kqueue()
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(kqueueFD)

	// Specify the events we want to get hints about and set the socket on which
	var socketServerEvent syscall.Kevent_t = syscall.Kevent_t{
		Ident:  uint64(serverFD),
		Filter: syscall.EVFILT_READ,
		Flags:  syscall.EV_ADD | syscall.EV_ENABLE,
	}

	// listen to read events on the Server itself
	if _, err = syscall.Kevent(
		kqueueFD,
		[]syscall.Kevent_t{socketServerEvent},
		nil,
		nil,
	); err != nil {
		return err
	}

	for {
		// see if any FD is ready for an IO
		nevents, e := syscall.Kevent(
			kqueueFD,
			nil,
			events[:],
			nil,
		)
		if e != nil {
			continue
		}

		for i := 0; i < nevents; i++ {
			// if socket server is ready for an IO
			if int(events[i].Ident) == serverFD {
				// accept the incoming connection from a client
				fd, _, err := syscall.Accept(serverFD)
				if err != nil {
					log.Panicln("err", err)
					continue
				}

				// increase the number of concurrent clients count
				con_clients++
				log.Println("client connected with address:", "concurrent clients", con_clients)
				syscall.SetNonblock(fd, true)

				// add this new TCP connection to be monitored
				var socketClientEvent syscall.Kevent_t = syscall.Kevent_t{
					Ident:  uint64(fd),
					Filter: syscall.EVFILT_READ,
					Flags:  syscall.EV_ADD | syscall.EV_ENABLE,
				}

				if _, err := syscall.Kevent(
					kqueueFD,
					[]syscall.Kevent_t{socketClientEvent},
					nil,
					nil,
				); err != nil {
					log.Fatal(err)
				}
			} else {
				comm := core.FDComm{Fd: int(events[i].Ident)}
				cmd, err := readCommand(comm)
				if err != nil {
					syscall.Close(int(events[i].Ident))
					con_clients -= 1
					log.Println("client connected with address:", "concurrent clients", con_clients)
					continue
				}

				respond(cmd, comm)
			}
		}
	}
}
