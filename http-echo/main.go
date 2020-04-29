package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/tobert/pcstat"
)

var (
	listenFlag = flag.String("listen", ":5678", "address and port to listen")
	textFlag   = flag.String("text", "", "text to put on the webpage")

	// stdoutW and stderrW are for overriding in test.
	stdoutW = os.Stdout
	stderrW = os.Stderr
)

const (
	httpHeaderAppName    string = "X-App-Name"
	httpHeaderAppVersion string = "X-App-Version"

	httpLogDateFormat string = "2006/01/02 15:04:05"
	httpLogFormat     string = "%v %s %s \"%s %s %s\" %d %d \"%s\" %v\n"
)

type SendJob struct {
	conn    net.Conn
	content string
}

type ReadDone struct {
	conn     net.Conn
	fileName string
}

// withAppHeaders adds application headers such as X-App-Version and X-App-Name.
// func withAppHeaders(h http.HandlerFunc) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Set(httpHeaderAppName)
// 		h(w, r)
// 	}
// }

// metaResponseWriter is a response writer that saves information about the
// response for logging.
// type metaResponseWriter struct {
// 	writer http.ResponseWriter
// 	status int
// 	length int
// }

// // Header implements the http.ResponseWriter interface.
// func (w *metaResponseWriter) Header() http.Header {
// 	return w.writer.Header()
// }

// // WriteHeader implements the http.ResponseWriter interface.
// func (w *metaResponseWriter) WriteHeader(s int) {
// 	w.status = s
// 	w.writer.WriteHeader(s)
// }

// // Write implements the http.ResponseWriter interface.
// func (w *metaResponseWriter) Write(b []byte) (int, error) {
// 	if w.status == 0 {
// 		w.status = http.StatusOK
// 	}
// 	w.length = len(b)
// 	return w.writer.Write(b)
// }

// // httpLog accepts an io object and logs the request and response objects to the
// // given io.Writer.
// func httpLog(out io.Writer, h http.HandlerFunc) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		var mrw metaResponseWriter
// 		mrw.writer = w

// 		defer func(start time.Time) {
// 			status := mrw.status
// 			length := mrw.length
// 			end := time.Now()
// 			dur := end.Sub(start)
// 			fmt.Fprintf(out, httpLogFormat,
// 				end.Format(httpLogDateFormat),
// 				r.Host, r.RemoteAddr, r.Method, r.URL.Path, r.Proto,
// 				status, length, r.UserAgent(), dur)
// 		}(time.Now())

// 		h(&mrw, r)
// 	}
// }

func acceptConn(listen net.Listener, connDone chan<- net.Conn) {
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err.Error())
			os.Exit(1)
		}
		connDone <- conn
	}
}

// Read requests from the client
func readreq(conn net.Conn, strRet chan<- ReadDone) {
	var buf = make([]byte, 1024)
	for {
		len, err := conn.Read(buf)

		if err != nil {
			fmt.Println("Error reading:", err.Error())
			break
		}

		s := string(buf[:len])
		fmt.Println("Request:", s)
		fmt.Println("Length =", binary.Size(buf))
		break
	}
	fmt.Println("exited str ret")
	strRet <- ReadDone{conn, string(buf)}
	fmt.Println("exited read Done")
}

func findFile(job ReadDone, content chan<- SendJob) {
	fmt.Println("entered find file")
	cont, err := ioutil.ReadFile(strings.Fields(job.fileName)[1][1:])
	if err != nil {
		fmt.Println(err)
		return
	}
	content <- SendJob{job.conn, string(cont)}
	fmt.Println("exited find file")
}

func send(job SendJob) {
	fmt.Println("entered Send Job")
	// should write response header using writev() and will finish soon
	bufwrite := []byte(job.content)
	job.conn.Write(bufwrite)
	job.conn.Close()
	fmt.Println("sent job")
}

// Check if a file with the given filename is all in memory
// Return true by default when error occurs
func inMemory(job ReadDone) (bool, error) {
	name := strings.Fields(job.fileName)[1][1:]
	if len(name) == 0 {
		return true, nil
	}
	f, openErr := os.Open(name)
	stat, statErr := f.Stat()
	if statErr != nil {
		fmt.Println(statErr)
		return true, statErr
	}
	size := stat.Size()
	if openErr != nil {
		fmt.Println(openErr)
		return true, openErr
	}
	res, err := pcstat.FileMincore(f, size)
	if err != nil {
		fmt.Println(err)
		return true, err
	}
	// data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	// Only when all pages are in memory will we use the main process to handle it
	for _, inMem := range res {
		if !inMem {
			return false, nil
		}
	}
	return true, nil
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		fmt.Fprintln(stderrW, "Too many arguments!")
		os.Exit(127)
	}

	var connDone = make(chan net.Conn, 100)
	var fileName = make(chan ReadDone, 1)
	var content = make(chan SendJob, 1)
	var Wcontent = make(chan SendJob, 100)

	l, err := net.Listen("tcp", ":8080")

	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	// Close the listener when this application closes
	defer l.Close()

	fmt.Println("Listening on localhost:8080")
	go acceptConn(l, connDone)

	for {
		select {
		case conn := <-connDone:
			readreq(conn, fileName)
		case buffer := <-fileName:
			fmt.Println("got read Done")
			inMem, err := inMemory(buffer)
			if err != nil {
				fmt.Println("Error checking file in memory or not")
				break
			}
			if inMem {
				fmt.Println("In memory")
				findFile(buffer, content)
			} else {
				fmt.Println("Not in memory")
				go findFile(buffer, Wcontent)
			}
		case sendContent := <-content:
			send(sendContent)
		case sendContent := <-Wcontent:
			go send(sendContent)
		}
	}

	// Flag gets printed as a page
	// mux := http.NewServeMux()
	// mux.HandleFunc("/", httpLog(stdoutW, withAppHeaders(httpEcho(*textFlag))))

	// // Health endpoint
	// mux.HandleFunc("/health", withAppHeaders(httpHealth()))

	// server := &http.Server{
	// 	Addr:    *listenFlag,
	// 	Handler: mux,
	// }
	// serverCh := make(chan struct{})
	// go func() {
	// 	log.Printf("[INFO] server is listening on %s\n", *listenFlag)
	// 	if err := server.ListenAndServe(); err != http.ErrServerClosed {
	// 		log.Fatalf("[ERR] server exited with: %s", err)
	// 	}
	// 	close(serverCh)
	// }()

	// signalCh := make(chan os.Signal, 1)
	// signal.Notify(signalCh, os.Interrupt)

	// // Wait for interrupt
	// <-signalCh

	// log.Printf("[INFO] received interrupt, shutting down...")
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	// if err := server.Shutdown(ctx); err != nil {
	// 	log.Fatalf("[ERR] failed to shutdown server: %s", err)
	// }

	// // If we got this far, it was an interrupt, so don't exit cleanly
	// os.Exit(2)
}

// func httpEcho(v string) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintln(w, v)
// 	}
// }

// func httpHealth() http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintln(w, `{"status":"ok"}`)
// 	}
// }
