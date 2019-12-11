package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"

	_ "github.com/lib/pq"
)

type PostgresqlCredentials struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port     int    `json:"port"`
	Database string `json:"name"`
}

// struct for reading env
type VCAPServices struct {
	PostgreSQL []struct {
		Credentials PostgresqlCredentials `json:"credentials"`
	} `json:"a9s-postgresql10"`
}

type BlogPost struct {
	ID          int
	Title       string
	Description string
}

// template store
var templates map[string]*template.Template

// fill template store
func initTemplates() {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	templates["index"] = template.Must(template.ParseFiles("templates/index.html", "templates/base.html"))
	templates["new"] = template.Must(template.ParseFiles("templates/new.html", "templates/base.html"))
}

func initDatabase() {
	client, err := NewClient()
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}
	defer client.Close()

	client.Exec("CREATE TABLE posts(id SERIAL, title varchar(256), description varchar(1024))")
}

func createCredentials() (PostgresqlCredentials, error) {
	// Kubernetes
	if os.Getenv("VCAP_SERVICES") == "" {
		host := os.Getenv("POSTGRESQL_HOST")
		if len(host) < 1 {
			err := fmt.Errorf("Environment variable POSTGRESQL_HOST missing.")
			log.Println(err)
			return PostgresqlCredentials{}, err
		}
		username := os.Getenv("POSTGRESQL_USERNAME")
		if len(username) < 1 {
			err := fmt.Errorf("Environment variable POSTGRESQL_USERNAME missing.")
			log.Println(err)
			return PostgresqlCredentials{}, err
		}
		password := os.Getenv("POSTGRESQL_PASSWORD")
		if len(password) < 1 {
			err := fmt.Errorf("Environment variable POSTGRESQL_PASSWORD missing.")
			log.Println(err)
			return PostgresqlCredentials{}, err
		}
		portStr := os.Getenv("POSTGRESQL_PORT")
		if len(portStr) < 1 {
			err := fmt.Errorf("Environment variable POSTGRESQL_PORT missing.")
			log.Println(err)
			return PostgresqlCredentials{}, err
		}
		database := os.Getenv("POSTGRESQL_DATABASE")
		if len(database) < 1 {
			err := fmt.Errorf("Environment variable POSTGRESQL_DATABASE missing.")
			log.Println(err)
			return PostgresqlCredentials{}, err
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Println(err)
			return PostgresqlCredentials{}, err
		}

		credentials := PostgresqlCredentials{
			Host:     host,
			Username: username,
			Password: password,
			Port:     port,
			Database: database,
		}
		return credentials, nil
	}

	// Cloud Foundry
	// no new read of the env var, the reason is the receiver loop
	var s VCAPServices
	err := json.Unmarshal([]byte(os.Getenv("VCAP_SERVICES")), &s)
	if err != nil {
		log.Println(err)
		return PostgresqlCredentials{}, err
	}

	return s.PostgreSQL[0].Credentials, nil
}

func renderTemplate(w http.ResponseWriter, name string, template string, viewModel interface{}) {
	tmpl, _ := templates[name]
	err := tmpl.ExecuteTemplate(w, template, viewModel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// NewClient ...
func NewClient() (*sql.DB, error) {
	credentials, err := createCredentials()
	if err != nil {
		return nil, err
	}

	connStr := "user=" + credentials.Username + " dbname=" + credentials.Database + " password=" + credentials.Password + " host=" + credentials.Host + " port=" + strconv.Itoa(credentials.Port) + " sslmode=disable"
	credentials.Password = "******"
	log.Printf("Connection to:\n%v\n", credentials)

	client, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return client, err
}

func clearDatabase(w http.ResponseWriter, r *http.Request) {
	client, err := NewClient()
	defer client.Close()
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}

	client.QueryRow(`DELETE FROM posts`)
	w.Write([]byte("OK"))
}

// create new Blog post
func createBlogPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	title := r.PostFormValue("title")
	description := r.PostFormValue("description")

	http.Redirect(w, r, "/", 302)

	// insert key value into service
	client, err := NewClient()
	defer client.Close()
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}
	var postID int
	err = client.QueryRow(`INSERT INTO posts(title, description) VALUES('` + title + `', '` + description + `') RETURNING id`).Scan(&postID)
	if err != nil {
		log.Printf("Failed to create new blog post with title %v and description %v ; err = %v", title, description, err)
		return
	}
	log.Println("Created new blog post entry with ID: " + strconv.Itoa(postID))
}

func newBlogPost(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "new", "base", nil)
}

func renderBlogPosts(w http.ResponseWriter, r *http.Request) {
	blogposts := make([]BlogPost, 0)

	client, err := NewClient()
	defer client.Close()
	if err != nil {
		log.Printf("Failed to create connection: %v\n", err)
	} else {
		log.Printf("Collecting blog posts.\n")
		// query entries
		rows, err := client.Query("SELECT id, title, description FROM posts")
		if err != nil {
			log.Printf("Failed to fetch blog posts, err = %v\n", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			var title string
			var description string
			err := rows.Scan(&id, &title, &description)
			if err == nil {
				blogposts = append(blogposts, BlogPost{ID: id, Title: title, Description: description})
			}
		}
	}

	renderTemplate(w, "index", "base", blogposts)
}

func main() {
	initTemplates()
	initDatabase()

	port := "3000"
	if port = os.Getenv("PORT"); len(port) == 0 {
		port = "3000"
	}

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	if os.Getenv("VCAP_SERVICES") == "" {
		dir, err = filepath.Abs("/app")
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("Public dir: %v\n", dir)

	fs := http.FileServer(http.Dir(path.Join(dir, "public")))
	http.Handle("/public/", http.StripPrefix("/public/", fs))
	http.HandleFunc("/", renderBlogPosts)
	http.HandleFunc("/blog-posts/new", newBlogPost)
	http.HandleFunc("/blog-posts/create", createBlogPost)
	http.HandleFunc("/clear", clearDatabase)

	log.Printf("Listening on :%v\n", port)
	http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}
