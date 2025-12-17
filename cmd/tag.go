package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/spf13/cobra"
)

// resolveCategoryID résout un nom ou ID de catégorie en ID
func resolveCategoryID(client *api.Client, nameOrID string) (int, error) {
	// Essayer d'abord de parser comme un ID
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return id, nil
	}

	// Sinon chercher par nom
	categories, err := client.ListCategories()
	if err != nil {
		return 0, err
	}

	nameOrID = strings.ToLower(nameOrID)
	for _, c := range categories {
		if strings.ToLower(c.Name) == nameOrID {
			return c.ID, nil
		}
	}

	return 0, fmt.Errorf("catégorie '%s' non trouvée", nameOrID)
}

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Gestion des catégories/tags",
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les catégories disponibles",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(cfg)

		categories, err := client.ListCategories()
		if err != nil {
			return err
		}

		for _, c := range categories {
			fmt.Printf("%d\t%s\t%s\n", c.ID, c.Color, c.Name)
		}
		return nil
	},
}

var recursive bool

var tagAddCmd = &cobra.Command{
	Use:   "add <category> <file_id>",
	Short: "Ajoute une catégorie à un fichier/répertoire",
	Long:  "Ajoute une catégorie (nom ou ID) à un fichier/répertoire",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(cfg)

		categoryName := args[0]
		categoryID, err := resolveCategoryID(client, categoryName)
		if err != nil {
			return err
		}

		// Récupérer le nom de la catégorie si on a passé un ID
		categories, _ := client.ListCategories()
		for _, c := range categories {
			if c.ID == categoryID {
				categoryName = c.Name
				break
			}
		}

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("file_id invalide: %s", args[1])
		}

		// Construire la liste des fichiers avec leurs noms
		type fileInfo struct {
			ID   int
			Name string
		}
		var files []fileInfo

		if recursive {
			// Récupérer d'abord le dossier racine
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})

			// Puis tous les enfants
			children, err := client.ListFilesRecursive(fileID)
			if err != nil {
				return err
			}
			for _, f := range children {
				files = append(files, fileInfo{ID: f.ID, Name: f.Name})
			}
			fmt.Printf("Application de [%s] à %d fichiers...\n", categoryName, len(files))
		} else {
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})
		}

		// Créer une map ID -> nom pour l'affichage
		fileNames := make(map[int]string)
		fileIDs := make([]int, 0, len(files))
		for _, f := range files {
			fileIDs = append(fileIDs, f.ID)
			fileNames[f.ID] = f.Name
		}

		// Batch par 50 pour éviter les timeouts
		batchSize := 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}
			batch := fileIDs[i:end]

			results, err := client.AddCategoryToFiles(categoryID, batch)
			if err != nil {
				return err
			}

			for _, r := range results {
				name := fileNames[r.ID]
				if r.Result {
					fmt.Printf("  [OK]   %s\n", name)
				} else {
					fmt.Printf("  [SKIP] %s (déjà taggé)\n", name)
				}
			}
		}

		fmt.Println("Fait.")
		return nil
	},
}

var tagRmCmd = &cobra.Command{
	Use:   "rm <category> <file_id>",
	Short: "Retire une catégorie d'un fichier/répertoire",
	Long:  "Retire une catégorie (nom ou ID) d'un fichier/répertoire",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(cfg)

		categoryName := args[0]
		categoryID, err := resolveCategoryID(client, categoryName)
		if err != nil {
			return err
		}

		// Récupérer le nom de la catégorie si on a passé un ID
		categories, _ := client.ListCategories()
		for _, c := range categories {
			if c.ID == categoryID {
				categoryName = c.Name
				break
			}
		}

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("file_id invalide: %s", args[1])
		}

		// Construire la liste des fichiers avec leurs noms
		type fileInfo struct {
			ID   int
			Name string
		}
		var files []fileInfo

		if recursive {
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})

			children, err := client.ListFilesRecursive(fileID)
			if err != nil {
				return err
			}
			for _, f := range children {
				files = append(files, fileInfo{ID: f.ID, Name: f.Name})
			}
			fmt.Printf("Retrait de [%s] de %d fichiers...\n", categoryName, len(files))
		} else {
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})
		}

		// Créer une map ID -> nom pour l'affichage
		fileNames := make(map[int]string)
		fileIDs := make([]int, 0, len(files))
		for _, f := range files {
			fileIDs = append(fileIDs, f.ID)
			fileNames[f.ID] = f.Name
		}

		// Batch par 50
		batchSize := 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}
			batch := fileIDs[i:end]

			results, err := client.RemoveCategoryFromFiles(categoryID, batch)
			if err != nil {
				return err
			}

			for _, r := range results {
				name := fileNames[r.ID]
				if r.Result {
					fmt.Printf("  [OK]   %s\n", name)
				} else {
					fmt.Printf("  [SKIP] %s (pas taggé)\n", name)
				}
			}
		}

		fmt.Println("Fait.")
		return nil
	},
}

func init() {
	tagAddCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Applique récursivement à tous les enfants")
	tagRmCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Retire récursivement de tous les enfants")

	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRmCmd)
	rootCmd.AddCommand(tagCmd)
}