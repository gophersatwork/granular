# Granular: High-Performance Incremental File Cache Library for Go

This document provides an architectural overview of the Granular file cache library, including entity relationships, file structure, and execution flow.

## Project Entities and Relationships

```
                                +----------------+
                                |     Cache      |
                                +----------------+
                                | - root         |
                                | - hash         |
                                | - mu           |
                                | - fs           |
                                +----------------+
                                | + Get()        |
                                | + Store()      |
                                | + GetFile()    |
                                | + GetData()    |
                                | + Clear()      |
                                | + Remove()     |
                                +----------------+
                                        |
                                        |
                +------------------------+------------------------+
                |                        |                        |
                v                        v                        v
        +----------------+      +----------------+      +----------------+
        |     Input      |      |    Manifest    |      |     Result     |
        +----------------+      +----------------+      +----------------+
        | + Hash()       |      | - KeyHash      |      | - Path         |
        | + String()     |      | - InputDescs   |      | - Metadata     |
        +----------------+      | - ExtraData    |      +----------------+
                |               | - OutputFiles  |
                |               | - OutputData   |
                |               | - OutputMeta   |
                |               | - OutputHash   |
                |               | - CreatedAt    |
                |               | - AccessedAt   |
                |               | - Description  |
                |               +----------------+
                |
    +-----------+-----------+-----------+-----------+
    |           |           |           |           |
    v           v           v           v           v
+--------+ +--------+ +--------+ +--------+ +--------+
|FileInput| |GlobInput| |DirInput| |RawInput| |  Key   |
+--------+ +--------+ +--------+ +--------+ +--------+
|+ Path  | |+ Pattern| |+ Path   | |+ Data  | |+ Inputs|
+--------+ +--------+ |+ Exclude | |+ Name  | |+ Extra |
                      +--------+ +--------+ +--------+
```

## File Structure and Relationships

```
granular/
├── granular.go       # Core types and initialization
│   ├── Cache         # Main cache structure
│   ├── Input         # Interface for cache inputs
│   ├── Key           # Cache key structure
│   └── Option        # Configuration options
│
├── input.go          # Input implementations
│   ├── FileInput     # Single file input
│   ├── GlobInput     # Glob pattern input
│   ├── DirectoryInput # Directory input
│   └── RawInput      # Raw data input
│
├── hash.go           # Hashing functionality
│   └── hashFile      # Hash file content
│
├── manifest.go       # Manifest handling
│   ├── Manifest      # Manifest structure
│   ├── computeKeyHash # Calculate key hash
│   ├── saveManifest  # Save manifest to disk
│   └── loadManifest  # Load manifest from disk
│
└── operations.go     # Cache operations
    ├── Get           # Retrieve cached result
    ├── Store         # Store result in cache
    ├── GetFile       # Get specific file
    ├── GetData       # Get specific data
    ├── Clear         # Clear entire cache
    └── Remove        # Remove specific entry
```

## Cache Storage Layout

```
.cache/                  # Root cache directory
├── manifests/           # Manifest storage
│   ├── ab/              # First 2 chars of hash
│   │   └── abcdef1234.json  # Manifest with metadata
│   └── 12/
│       └── 123456abcd.json
└── objects/             # Cached artifacts
    ├── ab/              # First 2 chars of hash
    │   └── abcdef1234/  # Directory containing cached files
    │       ├── output.txt
    │       └── data.bin
    └── 12/
        └── 123456abcd/
            └── result.json
```

## Data Flow during execution

### Cache Hit Flow

```
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌─────────┐
│  Input  │     │   Key   │     │ computeHash │     │ Manifest│
│  Files  │────>│ Creation│────>│  (xxHash)   │────>│ Lookup  │
└─────────┘     └─────────┘     └─────────────┘     └────┬────┘
                                                         │
                                                         │
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌────▼────┐
│  Result │<────│ Return  │<────│  Update     │<────│ Manifest│
│         │     │ Cached  │     │ AccessedAt  │     │ Found   │
└─────────┘     └─────────┘     └─────────────┘     └─────────┘
```

### Cache Miss Flow

```
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌─────────┐
│  Input  │     │   Key   │     │ computeHash │     │ Manifest│
│  Files  │────>│ Creation│────>│  (xxHash)   │────>│ Lookup  │
└─────────┘     └─────────┘     └─────────────┘     └────┬────┘
                                                         │
                                                         │
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌────▼────┐
│ ErrCache│<────│ Return  │<────│             │<────│   Not   │
│  Miss   │     │  Error  │     │             │     │  Found  │
└─────────┘     └─────────┘     └─────────────┘     └─────────┘
```

### Store Flow

```
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌──────────┐
│  Input  │     │   Key   │     │ computeHash │     │ Create   │
│  Files  │────>│ Creation│────>│  (xxHash)   │────>│ Object   │
└─────────┘     └─────────┘     └─────────────┘     │ Directory│
                                                    └────┬─────┘
                                                         │
                                                         │
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌────▼────┐
│ Success │<────│ Return  │<────│   Save      │<────│  Copy   │
│         │     │ Result  │     │  Manifest   │<────│ Files   │
└─────────┘     └─────────┘     └─────────────┘     └─────────┘
```

## Performance Optimizations

1. **xxHash**: Uses xxHash instead of cryptographic hashes for 10x better performance
2. **Buffer Pooling**: Reuses buffers for file I/O to reduce GC pressure
3. **Deterministic Ordering**: Sorts inputs for consistent hashing
4. **Two-Level Directory**: Uses first 2 characters of hash for better filesystem distribution