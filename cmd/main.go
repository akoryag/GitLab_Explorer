package main

import (
	"html/template"
	"log"
	"net/http"

	app "gitLab-explorer/internal"
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
	mux.Handle("/pipeline-url", appCtx.PipelineURLHandler())
	mux.Handle("/tags", appCtx.TagsHandler())
	mux.Handle("/tags/create", appCtx.CreateTagHandler())
	mux.Handle("/tags/delete", appCtx.DeleteTagHandler())

	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
