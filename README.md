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
ktools ls                       # Root of the drive
ktools ls 3                     # Contents of folder ID 3
ktools ls "Common documents"    # Contents by path
ktools ls "Common documents/RH" # Nested path
```

### Manage categories

```bash
# List available categories
ktools tag list

# Add a category (by name or ID)
ktools tag add Confidential 6088
ktools tag add 14 6088

# Add recursively to a folder and all its children
ktools tag add -r Internal 3

# Remove a category
ktools tag rm Confidential 6088
ktools tag rm -r Internal 3
```

## License

MIT
