package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"

	_ "github.com/lib/pq"
)

type PostgresqlCredentials struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Sslmode  string `json:"sslmode"`
	Password string `json:"password"`
	Port     int    `json:"port"`
	Database string `json:"name"`
}

// struct for reading env
type VCAPServices struct {
	PostgreSQL []struct {
		Credentials PostgresqlCredentials `json:"credentials"`
	} `json:"a9s-postgresql17-ms-1749548047"`
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
		sslmode := os.Getenv("POSTGRESQL_SSLMODE")
		if len(sslmode) < 1 {
			log.Println("Environment variable POSTGRESQL_SSLMODE missing. Using default of disabled")
			sslmode = "disable"
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Println(err)
			return PostgresqlCredentials{}, err
		}

		credentials := PostgresqlCredentials{
			Host:     host,
			Username: username,
			Sslmode:  sslmode,
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

	if s.PostgreSQL[0].Credentials.Sslmode == "" {
		s.PostgreSQL[0].Credentials.Sslmode = "enable"
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

	connStr := "user=" + credentials.Username + " dbname=" + credentials.Database + " password=" + credentials.Password + " host=" + credentials.Host + " port=" + strconv.Itoa(credentials.Port) + " sslmode=" + credentials.Sslmode
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
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}
	defer client.Close()

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
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}
	defer client.Close()
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
	defer client.Close()

	renderTemplate(w, "index", "base", blogposts)
}

func deleteBlogPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Printf("Failed to parse Form: %v", err)
		return
	}
	postID := r.PostFormValue("postID")

	client, err := NewClient()
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}
	defer client.Close()
	_, err = client.Exec(`DELETE FROM posts WHERE id =` + postID + `;`)
	if err != nil {
		log.Printf("Failed to delete post entry with ID: %v ; err = %v", postID, err)
		return
	}

	http.Redirect(w, r, "/", 303)

	log.Printf("Deleted blog post entry with ID: %v\n", postID)

}

func main() {
	log.Println(runtime.Version())

	initTemplates()
	initDatabase()

	port := "3000"
	if port = os.Getenv("PORT"); len(port) == 0 {
		port = "3000"
	}

	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))

	http.HandleFunc("/", renderBlogPosts)
	http.HandleFunc("/blog-posts/new", newBlogPost)
	http.HandleFunc("/blog-posts/create", createBlogPost)
	http.HandleFunc("/blog-posts/delete", deleteBlogPost)
	http.HandleFunc("/clear", clearDatabase)

	log.Printf("Listening on :%v\n", port)
	http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}
