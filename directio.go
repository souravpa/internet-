/*
 * This function create a server that could serve both static and dynamic content.
 * It implement AMPED architecture based on the introduction in the paper introducing Flash server.
 * For each static file, if it's in memory, will be executed directly bt the main process.
 * Otherwise, a helper Goroutine will handle it.
 * All dynamic files will be handled directly by a helper Goroutines.
 * Its default fixed port is 8080.
 */
package main

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tobert/pcstat"
)

type ErrorCode struct {
	conn   net.Conn
	code   string
	cause  string
	msg    string
	detail string
}
type SendJob struct {
	conn       *net.TCPConn
	content    []byte
	fileName   string
	status     string
	is_dynamic bool
	errInfo    ErrorCode
}

type ReadDone struct {
	conn     *net.TCPConn
	fileName string
	errInfo  ErrorCode
}

// Accept connections from the client
func acceptConn(listen *net.TCPListener, connDone chan<- *net.TCPConn) {
	for {
		conn, err := listen.AcceptTCP()
		if err != nil {
			fmt.Println("Error accepting connection:", err.Error())
			os.Exit(1)
		}
		connDone <- conn
	}
}

// Read requests from the client
func readreq(conn *net.TCPConn, strRet chan<- ReadDone) {
	var buf = make([]byte, 1024)
	empty := ErrorCode{conn, "", "", "", ""}
	for {
		_, err := conn.Read(buf)

		if err != nil {
			e := ErrorCode{conn, "500", "Erro reading", "Internal Server Error", err.Error()}
			strRet <- ReadDone{conn, string(buf), e}
			return
		}
		// s := string(buf[:len])
		// fmt.Println("Request:")
		// fmt.Println(s)
		break
	}
	strRet <- ReadDone{conn, string(buf), empty}
}

// Open the file and memory-map it
func findFile(job ReadDone, content chan<- SendJob) {
	if job.errInfo.code != "" {
		content <- SendJob{job.conn, []byte{}, "", "", false, job.errInfo}
		return
	}
	method := strings.Fields(job.fileName)[0]
	if method != "GET" {
		e := ErrorCode{job.conn, "501", method, "Not Implemented", "Server does not implement this method"}
		content <- SendJob{job.conn, []byte{}, "", "", false, e}
		return
	}
	str := strings.Fields(job.fileName)
	name := str[1][1:]
	i := len(str) - 1
	for {
		if i < 0 {
			break
		}
		if strings.Compare(str[i], "Connection:") == 0 {
			break
		}
		i--
	}
	status := str[i+1]

	if strings.Count(name, "/") == len(name) {
		name = "text/home.html"
	}
	name = "content/" + name

	empty := ErrorCode{job.conn, "", "", "", ""}
	if strings.Contains(name, "cgi-bin") { // Dynamic content
		pos := strings.IndexByte(name, '?')
		cgi := name[pos+1:]
		name = name[:pos]
		stat, statErr := os.Stat(name)
		if statErr != nil {
			e := ErrorCode{job.conn, "403", name, "Forbidden", statErr.Error()}
			content <- SendJob{job.conn, []byte{}, "", "", false, e}
			return
		}
		if (!stat.Mode().IsRegular()) || (stat.Mode()&syscall.S_IXUSR == 0) {
			e := ErrorCode{job.conn, "403", name, "Forbidden", "Server couldn't read the file"}
			content <- SendJob{job.conn, []byte{}, "", "", false, e}
			return
		}
		content <- SendJob{job.conn, []byte(cgi), "./" + name, status, true, empty}
	} else { // Static content
		f, openErr := os.Open(name)
		if openErr != nil {
			e := ErrorCode{job.conn, "404", name, "Not Found", openErr.Error()}
			content <- SendJob{job.conn, []byte{}, "", "", false, e}
			return
		}
		stat, statErr := os.Stat(name)
		if statErr != nil {
			e := ErrorCode{job.conn, "403", name, "Forbidden", statErr.Error()}
			content <- SendJob{job.conn, []byte{}, "", "", false, e}
			f.Close()
			return
		}
		if (!stat.Mode().IsRegular()) || (stat.Mode()&syscall.S_IRUSR) == 0 {
			e := ErrorCode{job.conn, "403", name, "Forbidden", "Server couldn't read the file"}
			content <- SendJob{job.conn, []byte{}, "", "", false, e}
			f.Close()
			return
		}
		data := make([]byte, stat.Size())
		_, mmErr := f.Read(data)
		if mmErr != nil {
			f.Close()
			e := ErrorCode{job.conn, "500", name, "Internal Server Error", mmErr.Error()}
			content <- SendJob{job.conn, []byte{}, "", "", false, e}
			return
		}
		f.Close()
		content <- SendJob{job.conn, data, name, status, false, empty}
	}
}

// Analyse the file type to be used in the response
func getType(filename string) string {
	if strings.Contains(filename, ".html") ||
		strings.Contains(filename, ".htm") {
		return "text/html"
	} else if strings.Contains(filename, ".gif") {
		return "image/gif"
	} else if strings.Contains(filename, ".png") {
		return "image/png"
	} else if strings.Contains(filename, ".jpg") ||
		strings.Contains(filename, ".jpeg") {
		return "image/jpeg"
	} else if strings.Contains(filename, ".txt") {
		return "text/plain"
	} else if strings.Contains(filename, ".css") {
		return "text/css"
	} else if strings.Contains(filename, ".js") {
		return "application/javascript"
	} else if strings.Contains(filename, ".mp4") {
		return "video/mp4"
	} else if strings.Contains(filename, ".webm") {
		return "video/webm"
	} else if strings.Contains(filename, ".ogg") {
		return "video/ogg"
	} else if strings.Contains(filename, ".pdf") {
		return "application/pdf"
	} else {
		return "application/octet-stream"
	}
}

// Align all response headers on 32-byte boundaries
// and padding their lengths to be a multiple of 32 bytes (the size of cache lines)
func align(line string, last bool) []byte {
	alignment := 32
	bufs := bytes.NewBufferString(line)
	var remainder int
	var size int
	if last {
		remainder = (len(line) + 4) % alignment
		size = len(line) + alignment - remainder + 4
	} else {
		remainder = (len(line) + 2) % alignment
		size = len(line) + alignment - remainder + 2
	}
	for i := 0; i < alignment-remainder; i++ {
		bufs.WriteByte(' ')
	}
	if last {
		bufs.WriteString("\r\n")
	}
	bufs.WriteString("\r\n")
	res := make([]byte, size)
	bufs.Read(res)
	return res
}

// Send the response header and body to the client
func send(job SendJob, errorHandling chan<- ErrorCode) {
	if job.errInfo.code != "" {
		errorHandling <- job.errInfo
		return
	}
	name := job.fileName
	if !job.is_dynamic { // Static content
		date := time.Now().UTC().Format(http.TimeFormat)
		stat, statErr := os.Stat(name)
		if statErr != nil {
			syscall.Munmap(job.content)
			e := ErrorCode{job.conn, "403", name, "Forbidden", statErr.Error()}
			errorHandling <- e
			return
		}
		modified := stat.ModTime().Format(http.TimeFormat)
		size := strconv.FormatInt(stat.Size(), 10)
		filetype := getType(name)
		var header net.Buffers
		res := align("HTTP/1.1 200 OK", false)
		header = append(header, res)
		res = align("Server: Web Server\r\n", false)
		header = append(header, res)
		res = align("Date: "+date, false)
		header = append(header, res)
		res = align("Last-Modified: "+modified, false)
		header = append(header, res)
		res = align("Content-Length: "+size, false)
		header = append(header, res)
		res = align("Content-Address: "+name, false)
		header = append(header, res)
		res = align("Content-Type: "+filetype, false)
		header = append(header, res)
		res = align("Connection: "+job.status, true)
		header = append(header, res)
		_, whErr := header.WriteTo(job.conn)
		if whErr != nil {
			e := ErrorCode{job.conn, "500", name, "Internal Server Error", whErr.Error()}
			errorHandling <- e
			return
		}
		_, wbErr := job.conn.Write(job.content)
		if wbErr != nil {
			e := ErrorCode{job.conn, "500", name, "Internal Server Error", wbErr.Error()}
			errorHandling <- e
			return
		}
	} else {
		var header net.Buffers
		res := align("HTTP/1.1 200 OK", false)
		header = append(header, res)
		res = align("Server: Web Server\r\n", false)
		header = append(header, res)
		_, whErr := header.WriteTo(job.conn)
		if whErr != nil {
			e := ErrorCode{job.conn, "500", name, "Internal Server Error", whErr.Error()}
			errorHandling <- e
			return
		}
		cmd := exec.Command(name)
		newEnv := append(os.Environ(), "QUERY_STRING="+string(job.content))
		cmd.Env = newEnv
		out, _ := cmd.CombinedOutput()
		job.conn.Write(out)
	}
	job.conn.Close()
}

// Check if a file with the given filename is all in memory and return mapping
// Return true by default when error occurs
func inMemory(job ReadDone) (bool, error) {
	name := strings.Fields(job.fileName)[1][1:]
	if len(name) == 0 {
		name = "text/home.html"
	}
	name = "content/" + name
	if strings.Contains(name, "cgi-bin") { // Dynamic content
		return false, nil
	}
	stat, statErr := os.Stat(name)
	if statErr != nil {
		fmt.Println(statErr.Error())
		return true, statErr
	}
	f, openErr := os.Open(name)
	if openErr != nil {
		fmt.Println(openErr.Error())
		return true, openErr
	}
	size := stat.Size()
	res, mcErr := pcstat.FileMincore(f, size)
	if mcErr != nil {
		f.Close()
		fmt.Println(mcErr.Error())
		return true, mcErr
	}
	// Only when all pages are in memory will we use the main process to handle it
	for _, inMem := range res {
		if !inMem {
			return false, nil
		}
	}
	return true, nil
}

// Display error information on a single page
func printError(serverError ErrorCode) {
	var body net.Buffers
	/* Build the HTTP response body */
	line := align("<html><title>Server Error</title>", false)
	body = append(body, line)
	line = align(serverError.code+": "+serverError.msg, false)
	body = append(body, line)
	line = align("<p>"+serverError.detail, false)
	body = append(body, line)
	line = align("<hr><em>Web server</em>", false)
	body = append(body, line)

	var header net.Buffers
	date := time.Now().UTC().Format(http.TimeFormat)
	/* Print the HTTP response */
	res := align("HTTP/1.1 "+serverError.code+" "+serverError.msg, false)
	header = append(header, res)
	res = align("Date: "+date, false)
	header = append(header, res)
	res = align("Content-Type: text/html", true)
	header = append(header, res)
	fmt.Print("Error occurs")
	_, whErr := header.WriteTo(serverError.conn)

	if whErr != nil {
		serverError.conn.Close()
		fmt.Println(whErr.Error())
		return
	}
	_, wbErr := body.WriteTo(serverError.conn)
	if wbErr != nil {
		serverError.conn.Close()
		fmt.Println(wbErr.Error())
		return
	}

	serverError.conn.Close()
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Too many arguments!")
		os.Exit(127)
	}

	var connDone = make(chan *net.TCPConn, 100)
	var fileName = make(chan ReadDone, 100)
	var content = make(chan SendJob, 100)
	var Wcontent = make(chan SendJob, 100)
	var errorHandling = make(chan ErrorCode, 100)

	addr, aErr := net.ResolveTCPAddr("tcp", "localhost:8080")
	if aErr != nil {
		fmt.Println("Error generating TCP address:", aErr.Error())
		os.Exit(1)
	}
	l, lErr := net.ListenTCP("tcp", addr)
	if lErr != nil {
		fmt.Println("Error listening:", lErr.Error())
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
			inMem, _ := inMemory(buffer)
			if inMem {
				findFile(buffer, content)
			} else {
				go findFile(buffer, content)
			}
		case sendContent := <-content:
			send(sendContent, errorHandling)
		case sendContent := <-Wcontent:
			go send(sendContent, errorHandling)
		case serverError := <-errorHandling:
			printError(serverError)
		}
	}
}
