# ktools

CLI tool to manage files on Infomaniak kDrive.

## Installation

```bash
go install github.com/gfaivre/ktools@latest
```

Or from source:

```bash
git clone https://github.com/gfaivre/ktools.git
cd ktools
go build -o ktools .
```

## Configuration

Create the file `~/.config/ktools/config.yaml`:

```yaml
api_token: "YOUR_API_TOKEN"
drive_id: YOUR_DRIVE_ID
```

- **api_token**: create at https://manager.infomaniak.com/v3/ng/accounts/token/list (scope `kdrive`)
- **drive_id**: visible in the URL https://drive.infomaniak.com/app/drive/[ID]/files

Alternative environment variables:
- `KTOOLS_API_TOKEN`
- `KTOOLS_DRIVE_ID`

## Usage

### Global flags

```bash
ktools -v <command>   # Verbose mode (debug logs)
```

### List files

```bash
ktools ls                              # Root of the drive
ktools ls 3                            # Contents of folder ID 3
ktools ls "Common documents"           # Contents by path
ktools ls "Common documents/Invoices"  # Nested path
```

Example output:

```
TYPE    MODIFIED            ID      NAME
dir     2025-01-15 09:30    3       Common documents
dir     2025-01-10 14:22    7       Private
```

```
TYPE    MODIFIED            ID      NAME
dir     2025-01-12 11:45    42      Invoices
dir     2025-01-08 16:30    51      Contracts
file    2025-01-05 10:15    63      guidelines.pdf
```

### Manage categories

```bash
# List available categories
ktools tag list
```

Example output:

```
ID      COLOR       NAME
14      ██ #c27c0e  Confidential
15      ██ #f7dd75  Internal
16      ██ #4caf50  Public
```

```bash
# Add a category (by name or ID) to a file/folder (by ID or path)
ktools tag add Confidential 42
ktools tag add 14 "Common documents/Invoices"

# Add recursively to a folder and all its children
ktools tag add -r Internal 3
ktools tag add -r Internal "Common documents"

# Remove a category
ktools tag rm Confidential 42
ktools tag rm -r Internal "Common documents"
```

### Scan directories

Find directories with many files or high storage usage:

```bash
# Scan from root, show top 10 by size
ktools scan

# Scan a specific folder (by ID or path)
ktools scan 3
ktools scan "Common documents"

# Show top 20 directories
ktools scan -n 20

# Show only directories with >= 50 files
ktools scan -t 50

# Sort by file count instead of size
ktools scan -s files

# Show all directories (no filtering)
ktools scan -a
```

Example output:

```text
FILES  SIZE      %      ID    NAME
156    1.2 GB    45.2%  42    Invoices
89     856.3 MB  32.1%  51    Archives
45     312.5 MB  11.7%  63    Projects

Total: 1245 files, 89 directories, 2.7 GB
```

Flags:

- `-n, --top N`: Show top N directories (default: 10, 0 = unlimited)
- `-t, --threshold N`: Minimum file count threshold (default: 100)
- `-s, --sort TYPE`: Sort by `size` (default) or `files`
- `-a, --all`: Show all directories (no filtering)

### Find stale files

Identify old files for retention review:

```bash
# Find files not modified in 2 years (default)
ktools stale

# Scan a specific folder
ktools stale "Common documents"

# Custom age threshold (2 years, 6 months, 90 days)
ktools stale -a 6m
ktools stale -a 3y

# Show top 50 largest stale files
ktools stale -n 50

# Only files larger than 1 MB
ktools stale -m 1048576
```

Example output:

```text
Age distribution:

RANGE        FILES  %      SIZE      %
< 6 months   245    19.7%  1.2 GB    44.4%
6m - 1 year  312    25.1%  856 MB    31.7%
1 - 2 years  421    33.8%  412 MB    15.3%
2 - 3 years  156    12.5%  156 MB    5.8%
3 - 5 years  89     7.2%   62 MB     2.3%
> 5 years    21     1.7%   12 MB     0.4%

Files not modified since 2y:

AGE     SIZE      MODIFIED    ID     NAME
3y 2m   45.2 MB   2021-10-15  1234   old_report.pdf
2y 8m   12.1 MB   2022-02-20  5678   archive_2022.zip

Total: 266 files, 230 MB (out of 1244 files, 2.7 GB)
```

Flags:

- `-a, --age`: Minimum age threshold (default: `2y`, formats: `2y`, `6m`, `90d`)
- `-n, --top N`: Show top N files (default: 20, 0 = unlimited)
- `-m, --min-size`: Minimum file size in bytes

## License

MIT
