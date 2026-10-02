package packages

import (
	"encoding/json"
	"os"
	"sync"
)

type Package struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Volume   int    `json:"volume"`   // in GB
	Duration int    `json:"duration"` // in days
}

func GetPredefinedPackages() []Package {
	return []Package{
		{ID: "p1", Name: "پکیج ۱ ماهه (۳۰ روز)", Volume: 50, Duration: 30},
		{ID: "p2", Name: "پکیج ۲ ماهه (۶۰ روز)", Volume: 100, Duration: 60},
		{ID: "p3", Name: "پکیج ۳ ماهه (۹۰ روز)", Volume: 150, Duration: 90},
		{ID: "p4", Name: "پکیج ۱۲ ماهه (۱ سال)", Volume: 500, Duration: 365},
	}
}

var (
	filePath = "packages.json"
	mu       sync.Mutex
)

func LoadPackages() ([]Package, error) {
	mu.Lock()
	defer mu.Unlock()

	var pkgs []Package
	if _, err := os.Stat(filePath); err == nil {
		data, err := os.ReadFile(filePath)
		if err == nil {
			json.Unmarshal(data, &pkgs)
		}
	}

	// Combine stored with predefined
	allPkgs := GetPredefinedPackages()
	allPkgs = append(allPkgs, pkgs...)
	return allPkgs, nil
}

func SavePackages(pkgs []Package) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(pkgs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func AddPackage(pkg Package) error {
	pkgs, err := LoadPackages()
	if err != nil {
		return err
	}
	pkgs = append(pkgs, pkg)
	return SavePackages(pkgs)
}

func GetPackage(id string) (*Package, error) {
	pkgs, err := LoadPackages()
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, nil
}

func UpdatePackage(pkg Package) error {
	pkgs, err := LoadPackages()
	if err != nil {
		return err
	}
	for i, p := range pkgs {
		if p.ID == pkg.ID {
			pkgs[i] = pkg
			return SavePackages(pkgs)
		}
	}
	return nil
}

func DeletePackage(id string) error {
	pkgs, err := LoadPackages()
	if err != nil {
		return err
	}
	newPkgs := []Package{}
	for _, p := range pkgs {
		if p.ID != id {
			newPkgs = append(newPkgs, p)
		}
	}
	return SavePackages(newPkgs)
}
