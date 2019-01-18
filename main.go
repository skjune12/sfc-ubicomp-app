package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	_ "github.com/mattn/go-sqlite3"
)

const timeFormat = "2006-01-02 15:04:05"

type App struct {
	Router *mux.Router
	// DB     *sql.DB
}

type Item struct {
	Id          int    `json:"id,omitempty"`
	Timestamp   string `json:"timestamp"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Barcode     string `json:"Barcode,omitempty"`
	// Deleted     bool   `json:"deleted,omitempty"`
}

func (a *App) Initialize() {
	a.Router = mux.NewRouter()
	a.InitializeDB()
	a.InitializeHandlers()
}

func (a *App) InitializeHandlers() {
	a.Router.HandleFunc("/", a.GetItems).Methods("GET")
	a.Router.HandleFunc("/register", a.Register).Methods("GET")
	a.Router.HandleFunc("/edit/{id}", a.Edit).Methods("GET")
	a.Router.HandleFunc("/item", a.CreateNewItem).Methods("POST")
	a.Router.HandleFunc("/item/{id}", a.GetItem).Methods("GET")
	a.Router.HandleFunc("/item/{id}", a.UpdateItem).Methods("POST", "PUT")
	a.Router.HandleFunc("/item/{id}", a.DeleteItem).Methods("DELETE")
}

func (a *App) InitializeDB() {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS "items" (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"timestamp" TEXT NOT NULL,
		"name" TEXT NOT NULL,
		"description" TEXT NULL,
		"owner" TEXT NOT NULL,
		"barcode" TEXT NULL)`)

	if err != nil {
		log.Fatal("CREATE TABLE: ", err)
	}
}

func (a *App) Run(addr string) {
	loggedRouter := handlers.LoggingHandler(os.Stdout, a.Router)
	log.Printf("Server Listen on Port %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, loggedRouter))
}

// 新規登録ページ
func (a *App) Register(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("./static/register.html")
	if err != nil {
		log.Fatal("os.Open:", err)
	}

	fmt.Fprintf(w, string(data))
}

// 編集ページ
func (a *App) Edit(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	id, err := strconv.Atoi(params["id"])
	if err != nil {
		fmt.Fprintln(w, "invalid id")
	}

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	row := db.QueryRow(
		`SELECT * FROM items where ID=?`,
		id,
	)

	var item Item

	err = row.Scan(&item.Id, &item.Timestamp, &item.Name, &item.Description, &item.Owner, &item.Barcode)
	switch {
	case err == sql.ErrNoRows:
		fmt.Fprintf(w, "Not Found")

	case err != nil:
		log.Fatal(err)

	// 見つかったら、テンプレートを描画
	default:
		t := template.Must(template.ParseFiles("templates/edit.tmpl"))

		if err := t.ExecuteTemplate(w, "edit.tmpl", item); err != nil {
			log.Fatal(err)
		}
	}
}

// アイテムの一覧を取得
func (a *App) GetItems(w http.ResponseWriter, r *http.Request) {
	items := []Item{}

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT * from items`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var item Item
		err = rows.Scan(&item.Id, &item.Timestamp, &item.Name, &item.Description, &item.Owner, &item.Barcode)
		if err != nil {
			log.Fatal(err)
		}

		items = append(items, item)
	}

	t := template.Must(template.ParseFiles("templates/index.tmpl"))
	if err := t.ExecuteTemplate(w, "index.tmpl", items); err != nil {
		log.Fatal(err)
	}
}

// アイテムを取得
func (a *App) GetItem(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	id, err := strconv.Atoi(params["id"])
	if err != nil {
		fmt.Fprintln(w, "invalid id")
	}

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	row := db.QueryRow(
		`SELECT * FROM items where ID=?`,
		id,
	)

	var item Item

	err = row.Scan(&item.Id, &item.Timestamp, &item.Name, &item.Description, &item.Owner, &item.Barcode)
	switch {
	case err == sql.ErrNoRows:
		fmt.Fprintf(w, "Not Found")

	case err != nil:
		log.Fatal(err)

	// 見つかったら、テンプレートを描画
	default:
		t := template.Must(template.ParseFiles("templates/detail.tmpl"))

		if err := t.ExecuteTemplate(w, "detail.tmpl", item); err != nil {
			log.Fatal(err)
		}
	}
}

// CreateNewItem 新規にアイテムを作成
func (a *App) CreateNewItem(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var url string

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	row := db.QueryRow(`SELECT id FROM items DESC LIMIT 1`)

	var itemID int

	err = row.Scan(&itemID)
	switch {
	case err == sql.ErrNoRows:
		fmt.Fprintf(w, "Not Found")

	case err != nil:
		log.Fatal(err)

	default:
		url = fmt.Sprintf("http://%s%s/%d\n", r.Host, r.URL, itemID)
	}

	if len(url) < 1 {
		fmt.Fprintln(w, "Missing needed parameter 'm'.")
		return
	}

	newItem := Item{
		// Id:          itemID,
		Timestamp:   time.Now().Format(timeFormat),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Owner:       r.FormValue("owner"),
		Barcode:     a.GenerateBarcode(url),
		// Deleted:     false,
	}

	_, err = db.Exec(`INSERT INTO "items" (
			"timestamp",
			"name",
			"description",
			"owner",
			"barcode"
		) VALUES (?, ?, ?, ?, ?)`,
		newItem.Timestamp,
		newItem.Name,
		newItem.Description,
		newItem.Owner,
		newItem.Barcode,
		//newItem.Deleted,
	)

	if err != nil {
		log.Fatal("db.Exec:", err)
	}

	fmt.Fprintf(w, "success")
}

// UpdateItem 選択したアイテムを更新する
func (a *App) UpdateItem(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	id, err := strconv.Atoi(params["id"])
	if err != nil {
		fmt.Fprintf(w, "invalid id")
		return
	}

	if id < 0 {
		fmt.Fprintf(w, "invalid id")
		return
	}

	url := fmt.Sprintf("http://%s%s/%d\n", r.Host, r.URL, id)

	if len(url) < 1 {
		fmt.Fprintln(w, "Missing needed parameter 'm'.")
		return
	}

	newItem := Item{
		Id:          id,
		Timestamp:   time.Now().Format(timeFormat),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Owner:       r.FormValue("owner"),
		Barcode:     a.GenerateBarcode(url),
		// Deleted:     false,
	}

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	_, err = db.Exec(
		`UPDATE items SET timestamp=?, name=?, description=?, owner=? WHERE id=?`,
		newItem.Timestamp,
		newItem.Name,
		newItem.Description,
		newItem.Owner,
		newItem.Id,
	)

	if err != nil {
		log.Fatal("db.Exec:", err)
	}

	fmt.Fprintf(w, "success (id = %d)", newItem.Id)
}

func (a *App) GenerateBarcode(content string) string {
	bc, _ := qr.Encode(content, qr.L, qr.Auto)
	bc, _ = barcode.Scale(bc, 512, 512)

	buf := new(bytes.Buffer)
	base := base64.NewEncoder(base64.StdEncoding, buf)
	png.Encode(base, bc)
	base.Close()

	return buf.String()
}

// DeleteItem 選択したアイテムを削除する
func (a *App) DeleteItem(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	id, err := strconv.Atoi(params["id"])
	if err != nil {
		fmt.Fprintf(w, "invalid id")
		return
	}

	if id < 0 {
		fmt.Fprintf(w, "invalid id")
		return
	}

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	_, err = db.Exec(
		`DELETE FROM items WHERE id=?`,
	)

	if err != nil {
		log.Fatal("db.Exec:", err)
	}

	fmt.Fprintf(w, "success")
}

var app App

const dbName = "app.sqlite3"

func init() {
	app.Initialize()
}

// Server Listen on Port :8080
func main() {
	app.Run(":8080")
}
