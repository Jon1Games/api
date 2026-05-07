package database

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	DB         *sql.DB
	Driver     string
	TimeFrames map[string]int16
)

func Init() error {
	_ = godotenv.Load()
	db_host := os.Getenv("DB_HOST")
	if db_host == "" {
		return fmt.Errorf("environment variable 'DB_HOST' is not set")
	}
	db_port := os.Getenv("DB_PORT")
	if db_port == "" {
		return fmt.Errorf("environment variable 'DB_PORT' is not set")
	}
	db_name := os.Getenv("DB_NAME")
	if db_name == "" {
		return fmt.Errorf("environment variable 'DB_NAME' is not set")
	}
	db_user := os.Getenv("DB_USER")
	if db_user == "" {
		return fmt.Errorf("environment variable 'DB_USER' is not set")
	}
	db_pass := os.Getenv("DB_PASSWORD")
	if db_pass == "" {
		return fmt.Errorf("environment variable 'DB_PASSWORD' is not set")
	}
	driver := os.Getenv("DB_DRIVER")
	if driver == "" {
		return fmt.Errorf("environment variable 'DB_DRIVER' is not set, options are 'postgres' or 'mysql'")
	} else {
		Driver = driver
	}

	if err := connect(db_host, db_port, db_name, db_user, db_pass); err != nil {
		return err
	}
	if err := createTables(); err != nil {
		return err
	}
	if err := seedTimeFrames(); err != nil {
		return err
	}
	timeFrames, err := getTimeFrames()
	if err != nil {
		return err
	}
	TimeFrames = timeFrames

	return nil
}

func connect(addr string, port string, name string, user string, pass string) error {
	var dsn string
	switch Driver {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, addr, port, name)
	default:
		return fmt.Errorf("unsupported DB_DRIVER: %s, supported options are 'mysql'", Driver)
	}

	dbConn, err := sql.Open(Driver, dsn)
	if err != nil {
		return err
	} else {
		fmt.Printf("Successfully opened connection to database\n")
	}

	dbConn.SetConnMaxLifetime(time.Hour)
	dbConn.SetMaxOpenConns(25)
	dbConn.SetMaxIdleConns(25)

	if err := dbConn.Ping(); err != nil {
		dbConn.Close()
		return err
	}

	DB = dbConn
	return nil
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

func createTables() error {
	var createTableSQL []string
	switch Driver {
	case "mysql":
		createTableSQL = append(createTableSQL, `
			CREATE TABLE IF NOT EXISTS correspondent (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(64) NOT NULL
			)
		`)
		createTableSQL = append(createTableSQL, `
			CREATE TABLE IF NOT EXISTS sales_regulary (
				id INT AUTO_INCREMENT PRIMARY KEY,
				correspondent INT NOT NULL,
				name VARCHAR(64) NOT NULL,
				value DECIMAL(10, 2) NOT NULL,
				returning_months tinyint NOT NULL,
				FOREIGN KEY (correspondent) REFERENCES correspondent(id)
			)
		`)
		createTableSQL = append(createTableSQL, `
			CREATE TABLE IF NOT EXISTS timeFrame (
				id VARCHAR(64) PRIMARY KEY,
				name tinyint NOT NULL
			)
		`)
		createTableSQL = append(createTableSQL, `
			CREATE TABLE IF NOT EXISTS auth (
				id int PRIMARY KEY,
				value varchar(68) NOT NULL
			)
		`)
	}

	for _, query := range createTableSQL {
		if _, err := DB.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func seedTimeFrames() error {
	_, err := DB.Exec(`
		INSERT INTO timeFrame (id, name)
		VALUES
			('monthly', 1),
			('yearly', 12)
		ON DUPLICATE KEY UPDATE id = id
	`)
	return err
}

func getTimeFrames() (map[string]int16, error) {
	rows, err := DB.Query(`SELECT id, name FROM timeFrame`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	timeFrames := make(map[string]int16)
	for rows.Next() {
		var id string
		var name int16
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		timeFrames[id] = name
	}

	return timeFrames, nil
}
