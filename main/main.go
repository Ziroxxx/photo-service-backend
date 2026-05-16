package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"photo-service-back/app"
	"photo-service-back/config"
	accessdomain "photo-service-back/domain/access"
	competitiondomain "photo-service-back/domain/competition"
	photodomain "photo-service-back/domain/photo"
	"photo-service-back/domain/user"
	infraauth "photo-service-back/infra/auth"
	"photo-service-back/infra/imaging"
	infraocr "photo-service-back/infra/ocr"
	"photo-service-back/infra/postgres"
	objstorage "photo-service-back/infra/storage"
	"photo-service-back/transport/http/handlers"
	"photo-service-back/transport/http/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg := config.MustLoad()
	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	coverStorage, err := objstorage.NewMinioObjectStorage(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOUseSSL,
		cfg.MinIOCompetitionCoversBucket,
		cfg.MinIOPublicBaseURL,
	)
	if err != nil {
		log.Fatalf("failed to init minio cover client: %v", err)
	}

	if err := coverStorage.EnsureBucket(ctx); err != nil {
		log.Fatalf("failed to ensure minio cover bucket: %v", err)
	}

	photoStorage, err := objstorage.NewPhotoStorage(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOUseSSL,
		cfg.MinIOPhotoOriginalsBucket,
		cfg.MinIOPhotoDerivedBucket,
		cfg.MinIOPublicBaseURL,
	)
	if err != nil {
		log.Fatalf("failed to init minio photo client: %v", err)
	}

	if err := photoStorage.EnsureBuckets(ctx); err != nil {
		log.Fatalf("failed to ensure minio photo buckets: %v", err)
	}

	userRepo := postgres.NewUserRepo(db)
	refreshRepo := postgres.NewRefreshTokenRepo(db)
	competitionRepo := postgres.NewCompetitionRepo(db)
	accessRepo := postgres.NewAccessRepo(db)
	photoRepo := postgres.NewPhotoRepo(db)

	maintenanceHandler, err := handlers.NewMaintenanceHandler(
		db,
		cfg.AppEnv,
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOUseSSL,
		[]string{
			cfg.MinIOCompetitionCoversBucket,
			cfg.MinIOPhotoOriginalsBucket,
			cfg.MinIOPhotoDerivedBucket,
		},
	)
	if err != nil {
		log.Fatalf("failed to init maintenance handler: %v", err)
	}

	tokenManager := infraauth.NewTokenManager(
		cfg.JWTSecret,
		cfg.AccessTokenTTL,
		cfg.RefreshTokenTTL,
	)

	imageProcessor := imaging.NewProcessor(
		cfg.PhotoPreviewMaxWidth,
		cfg.PhotoPreviewMaxHeight,
		cfg.PhotoWatermarkText,
		cfg.PhotoJPEGQuality,
		cfg.PhotoWatermarkImagePath,
	)

	var bibRecognizer photodomain.BibRecognizer
	if cfg.OCRServiceURL != "" {
		ocrHTTPClient := &http.Client{
			Timeout: cfg.OCRServiceTimeout,
		}

		bibRecognizer = infraocr.NewClient(cfg.OCRServiceURL, ocrHTTPClient)
		log.Printf("ocr recognizer enabled: %s", cfg.OCRServiceURL)
	} else {
		log.Printf("ocr recognizer disabled: OCR_SERVICE_URL is empty")
	}

	authService := user.NewAuthService(userRepo, refreshRepo, tokenManager)
	userService := user.NewUserService(userRepo)
	competitionService := competitiondomain.NewService(competitionRepo, coverStorage)
	accessService := accessdomain.NewService(accessRepo, competitionRepo)
	photoService := photodomain.NewService(
		photoRepo,
		imageProcessor,
		photoStorage,
		accessService,
		competitionRepo,
		bibRecognizer,
	)

	authHandler := handlers.NewAuthHandler(authService, userService)
	userHandler := handlers.NewUserHandler(userService)
	competitionHandler := handlers.NewCompetitionHandler(competitionService)
	accessHandler := handlers.NewAccessHandler(accessService)
	photoHandler := handlers.NewPhotoHandler(photoService)

	authMiddleware := middleware.NewAuthMiddleware(tokenManager, userRepo)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	app.RegisterRoutes(r, app.HTTPDeps{
		AuthHandler:        authHandler,
		UserHandler:        userHandler,
		CompetitionHandler: competitionHandler,
		AccessHandler:      accessHandler,
		PhotoHandler:       photoHandler,
		MaintenanceHandler: maintenanceHandler,
		AuthMiddleware:     authMiddleware,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("server started on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
