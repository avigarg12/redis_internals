package server

import (
	"log"
	"net"
	"os"
	"redis_internals/config"
	"redis_internals/core"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func WaitForSignal(wg *sync.WaitGroup, sigs chan os.Signal) {
	defer wg.Done()
	<-sigs

	// if server is busy continue to wait
	for atomic.LoadInt32(&eStatus) == EngineStatus_BUSY {
	}

	// CRITICAL TO HANDLE
	// we donot want our server to go back to BUSY when the control flow is here
	atomic.StoreInt32(&eStatus, EngineStatus_SHUTTING_DOWN)

	core.Shutdown()
	os.Exit(0)
}

func RunAsyncTCPServer(wg *sync.WaitGroup) error {
	defer wg.Done()
	defer func() {
		atomic.StoreInt32(&eStatus, EngineStatus_SHUTTING_DOWN)
	}()

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

	// loop until the server is not shutting down
	for atomic.LoadInt32(&eStatus) != EngineStatus_SHUTTING_DOWN {

		// cron key expiration to be ran
		if time.Now().After(lastCronExecTime.Add(cronFrequency)) {
			core.DeleteExpiredKeys()
			lastCronExecTime = time.Now()
		}

		/*
			Say, the Engine triggered SHUTTING down when the control flow is here ->
			Current: Engine status == WAITING
			Update: Engine status = SHUTTING_DOWN
			then we have to exit
		*/

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

		// We dont want our server to back from SHUTTING DOWN to BUSY
		// If the engine status == SHUTTING_DOWN we want to exit
		// Hence the only legal transition is from WAITING to BUSY
		// mark engine as BUSY only when it is in the WAITING_STATE
		if !atomic.CompareAndSwapInt32(&eStatus, EngineStatus_WAITING, EngineStatus_BUSY) {
			// if swap unsuccessfull then the existing status is not WAITING, but something else
			switch eStatus {
			case EngineStatus_SHUTTING_DOWN:
				return nil
			}
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

				connectedClients[fd] = core.NewClient(fd)
				// log.Println("client connected with address:", "concurrent clients", con_clients)
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
				comm := connectedClients[int(events[i].Ident)]
				if comm == nil {
					continue
				}
				cmds, err := readCommands(comm)
				if err != nil {
					syscall.Close(int(events[i].Ident))
					delete(connectedClients, int(events[i].Ident))
					// log.Println("client connected with address:", "concurrent clients", con_clients)
					continue
				}

				respond(cmds, comm)
			}
		}
		// no contention as the signal handler is blocked until the engine is BUSY
		atomic.StoreInt32(&eStatus, EngineStatus_WAITING)
	}

	return nil
}
