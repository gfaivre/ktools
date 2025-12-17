# ktools

CLI pour gérer les fichiers sur Infomaniak kDrive.

## Installation

```bash
go install github.com/gfaivre/ktools@latest
```

Ou depuis les sources :

```bash
git clone https://github.com/gfaivre/ktools.git
cd ktools
go build -o ktools .
```

## Configuration

Créer le fichier `~/.config/ktools/config.yaml` :

```yaml
api_token: "VOTRE_TOKEN_API"
drive_id: YOUR_DRIVE_ID
```

- **api_token** : créer sur https://manager.infomaniak.com/v3/ng/accounts/token/list (scope `kdrive`)
- **drive_id** : visible dans l'URL https://drive.infomaniak.com/app/drive/[ID]/files

Variables d'environnement alternatives :
- `KTOOLS_API_TOKEN`
- `KTOOLS_DRIVE_ID`

## Utilisation

### Lister les fichiers

```bash
ktools ls           # Racine du drive
ktools ls 3         # Contenu du dossier ID 3
```

### Gérer les catégories

```bash
# Lister les catégories disponibles
ktools tag list

# Ajouter une catégorie (par nom ou ID)
ktools tag add Confidentiel 6088
ktools tag add 14 6088

# Ajouter récursivement à un dossier et tous ses enfants
ktools tag add -r Interne 3

# Retirer une catégorie
ktools tag rm Confidentiel 6088
ktools tag rm -r Interne 3
```

## Licence

MIT
