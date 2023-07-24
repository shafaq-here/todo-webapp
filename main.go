package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var rnd *renderer.Render
var todoCollection *mongo.Collection

const (
	dbname         string = "demo_todo"
	collectionName string = "todo"
	port           string = ":8080"
)

type todo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

func init() {
	rnd = renderer.New()

	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	checkErr(err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	checkErr(err)

	db := client.Database(dbname)
	todoCollection = db.Collection(collectionName)
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkErr(err)
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {
	todos := []todo{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := todoCollection.Find(ctx, bson.M{})
	checkErr(err)
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var t todo
		err := cursor.Decode(&t)
		checkErr(err)
		todos = append(todos, t)
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todos,
	})
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title is empty, cannot add todo",
		})
		return
	}

	t.ID = bson.TypeObjectID.String()
	t.CreatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := todoCollection.InsertOne(ctx, t)
	checkErr(err)

	rnd.JSON(w, http.StatusCreated, renderer.M{
		"message": "todo created successfully",
		"todo_id": t.ID,
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	// if bson.TypeObjectID.String() == id {
	// 	rnd.JSON(w, http.StatusBadRequest, renderer.M{
	// 		"message": "Invalid todo id",
	// 	})
	// 	return
	// }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := todoCollection.DeleteOne(ctx, bson.M{"_id": id})
	checkErr(err)

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "todo deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	// if bson.TypeObjectID.String() == id {
	// 	rnd.JSON(w, http.StatusBadRequest, renderer.M{
	// 		"message": "Invalid todo id",
	// 	})
	// 	return
	// }

	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "todo title empty",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := todoCollection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"title": t.Title, "completed": t.Completed}},
	)
	checkErr(err)

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "todo updated successfully",
	})
}

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandlers())

	server := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Listening on port", port)
		if err := server.ListenAndServe(); err != nil {
			log.Printf("listen: %v\n", err)
		}
	}()
	<-stopChan
	log.Println("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	server.Shutdown(ctx)
	defer cancel()
	log.Println("Server stopped gracefully")
}

func todoHandlers() http.Handler {
	rg := chi.NewRouter()

	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodos)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}
