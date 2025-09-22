package main

import (
	"html/template"
	"log"
	"net/http"

	app "GitLab_Explorer/internal"
)

func main() {
	tmpl, err := template.ParseFiles("templates/page.html")
	if err != nil {
		log.Fatal("Ошибка загрузки шаблона:", err)
	}

	appCtx := &app.AppContext{Tmpl: tmpl}

	mux := http.NewServeMux()
	mux.HandleFunc("/", appCtx.Handler)
	mux.HandleFunc("/logout", appCtx.LogoutHandler)
	mux.Handle("/pipeline", appCtx.PipelineHandler())
	mux.Handle("/job", appCtx.HandleJobAction())

	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
