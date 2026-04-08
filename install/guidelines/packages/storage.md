# Velocity Storage

## Drivers

- `local` - Local filesystem (default root: `./storage/app`)
- `s3` - AWS S3 bucket
- `memory` - In-memory (testing)

## Configuration

- `STORAGE_DRIVER` - local, s3, or memory
- `FILESYSTEM_LOCAL_ROOT` - Local storage root
- `AWS_BUCKET`, `AWS_DEFAULT_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` - S3 config

## Usage

```go
storage := services.Storage

// Store
err := storage.Put("uploads/photo.jpg", data)

// Retrieve
data, err := storage.Get("uploads/photo.jpg")

// Delete
err := storage.Delete("uploads/photo.jpg")

// Check existence
exists := storage.Exists("uploads/photo.jpg")
```

## Rules

- Never store uploaded files in the project root - use `./storage/app`
- Validate file types and sizes before storing
- Use S3 in production for durability and scalability
- Don't commit stored files to git - add `storage/app/` to `.gitignore`
