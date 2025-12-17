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
# Add a category (by name or ID)
ktools tag add Confidential 42
ktools tag add 14 42

# Add recursively to a folder and all its children
ktools tag add -r Internal 3

# Remove a category
ktools tag rm Confidential 42
ktools tag rm -r Internal 3
```

## License

MIT
