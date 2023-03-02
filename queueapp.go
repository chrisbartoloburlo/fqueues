package main

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"gorm.io/gorm"
  "gorm.io/driver/sqlite"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/skip2/go-qrcode"
	"encoding/base64"
)
type Ticket struct {
  Number uint `json:"Number"`
  IsActive bool `json:"isActive"`
}
var ticketsDb, err = gorm.Open(sqlite.Open("tickets.db"), &gorm.Config{})
var upgrader = websocket.Upgrader{} 

func serveTicket(w http.ResponseWriter, r *http.Request) {
	// if r.URL.Path != "/" {
	// 	http.Error(w, "404 not found.", http.StatusNotFound)
	// 	return
	// }

	switch r.Method {
	case "GET":		
		 http.ServeFile(w, r, "form.html")
	case "POST":
		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}
		var t = Ticket{}
		ticketsDb.Where(map[string]interface{}{}).Last(&t)
		ticketsDb.Create(&Ticket{Number: t.Number+1, IsActive: true})
		ticketsDb.Where(map[string]interface{}{}).Last(&t)
		ticketJSON, err := json.Marshal(t)
		if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(ticketJSON)
		// fmt.Fprintf(w, "Post from website! r.PostFrom = %v\n", r.PostForm)
	default:
		fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
	}
}

func filter(ts []Ticket, test func(Ticket) bool) (ret []Ticket) {
	for _, t := range ts {
		if test(t) {
			ret = append(ret, t)
		}
	}
	return
}

var isActiveFilter = func(t Ticket) bool { return t.IsActive }


var wsClients = make(map[string]*websocket.Conn)
func serveWS(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	var tickets []Ticket
  ticketsDb.Find(&tickets)
	var oldTickets []Ticket
	for {
		ticketsDb.Find(&tickets)
		if(!reflect.DeepEqual(tickets, oldTickets)) {
			oldTickets = tickets
			err = c.WriteJSON(filter(tickets, isActiveFilter))
			if err != nil {
				log.Println("write:", err)
			}
		}
	}
}

func generateQRCodeHtmlImageTag() string {
	websiteURL := "localhost:8080/ticket"
	imageSize := 256
	qrCodeImageData, taskError := qrcode.Encode(websiteURL, qrcode.High, imageSize)

	if taskError != nil {
		 log.Fatalln("Error generating QR code. ",taskError)
	}

	// Encode raw QR code data to base 64
	encodedData := base64.StdEncoding.EncodeToString(qrCodeImageData)

	return "<img src=\"data:image/png;base64, "+encodedData+"\">"
}

func serveQueue(response http.ResponseWriter, _ *http.Request) {

	// generate QR code HTML image tag
	qr_code_image_tag := generateQRCodeHtmlImageTag()
	// Set response header to Status Ok (200)
	response.WriteHeader(http.StatusOK)

	// HTML image with embedded QR code
	responsePageHtml := "<!DOCTYPE html><html><body><h1>Scan this to queue</h1>"+qr_code_image_tag+"</body></html>"

	// Send HTML response back to client
	_,_ = response.Write([]byte(responsePageHtml))
}


func main() {	
	ticketsDb.AutoMigrate(&Ticket{})
	ticketsDb.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Ticket{})
	ticketsDb.Create(&Ticket{Number: 0, IsActive: false})
	http.HandleFunc("/ticket", serveTicket)
	http.HandleFunc("/tickets", serveWS)

	http.HandleFunc("/queue", serveQueue)

	fmt.Printf("Starting server for testing HTTP POST...\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}