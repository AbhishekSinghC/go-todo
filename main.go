package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var collection *mongo.Collection

const (
	hostName       = "mongodb://localhost:27017"
	dbName         = "demo-todo"
	collectionName = "todo"
	port           = ":9000"
)

type todoModel struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Title     string        `bson:"title"         json:"title"`
	Completed bool          `bson:"completed"     json:"completed"`
	CreatedAt time.Time     `bson:"created_at"    json:"created_at"`
}

// ── init ─────────────────────────────────────────────────────────────
func init() {
	client, err := mongo.Connect(options.Client().ApplyURI(hostName))
	checkErr(err)

	// ping to verify connection is actually working
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = client.Ping(ctx, nil)
	checkErr(err)

	log.Println("✅ MongoDB connected!")
	collection = client.Database(dbName).Collection(collectionName)
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// ── main ─────────────────────────────────────────────────────────────
func main() {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE",
		AllowHeaders: "Content-Type",
	}))

	// serve UI
	app.Static("/", "./static")

	// api routes
	app.Get("/api", homeHandler)
	todoHandlers(app)

	// graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	go func() {
		log.Println("🚀 Server running at http://localhost" + port)
		if err := app.Listen(port); err != nil {
			log.Printf("Error: %s\n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down...")
	app.ShutdownWithTimeout(5 * time.Second)
	log.Println("Stopped ✅")
}

// ── Routes ────────────────────────────────────────────────────────────
func todoHandlers(app *fiber.App) {
	rg := app.Group("/todo")
	rg.Get("/", fetchTodos)
	rg.Post("/", createTodo)
	rg.Put("/:id", updateTodo)
	rg.Delete("/:id", deleteTodo)
}

func homeHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "todo api is running", "version": "1.0"})
}

// ── Handlers ──────────────────────────────────────────────────────────

// GET /todo/
func fetchTodos(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// sort by newest first
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer cursor.Close(ctx)

	todos := []todoModel{}
	if err := cursor.All(ctx, &todos); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// return empty array not null when no todos
	if todos == nil {
		todos = []todoModel{}
	}

	return c.JSON(fiber.Map{"data": todos})
}

// POST /todo/
func createTodo(c *fiber.Ctx) error {
	t := &todoModel{}

	if err := c.BodyParser(t); err != nil {
		return c.Status(422).JSON(fiber.Map{"error": "invalid request body"})
	}

	if t.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "title is required"})
	}

	t.ID        = bson.NewObjectID()
	t.Completed = false
	t.CreatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := collection.InsertOne(ctx, t); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{"message": "todo created successfully"})
}

// PUT /todo/:id
func updateTodo(c *fiber.Ctx) error {
	id, err := bson.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}

	t := &todoModel{}
	if err := c.BodyParser(t); err != nil {
		return c.Status(422).JSON(fiber.Map{"error": "invalid request body"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{
		"title":     t.Title,
		"completed": t.Completed,
	}}

	if _, err := collection.UpdateByID(ctx, id, update); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "todo updated successfully"})
}

// DELETE /todo/:id
func deleteTodo(c *fiber.Ctx) error {
	id, err := bson.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := collection.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "todo deleted successfully"})
}