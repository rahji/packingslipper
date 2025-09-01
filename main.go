package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	goshopify "github.com/bold-commerce/go-shopify/v4"
	"github.com/charmbracelet/log"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/signintech/gopdf"
	"gopkg.in/yaml.v2"
)

type CLIFlags struct {
	OutFilename     string `kong:"default='packingslip.pdf',name='outfile',help='Output PDF filename'"`
	OrderOffset     int    `kong:"default=0,name='offset',help='Offset from most recent order to retrieve'"`
	ConfigFilename  string `kong:"name='config',help='Configuration YAML file (default: ~/.config/packingslipper/configuration.yaml)'"`
	SecretsFilename string `kong:"name='secrets',help='Encrypted secrets YAML file (default: ~/.config/packingslipper/secrets.enc.yaml)'"`
	Verbose         bool   `kong:"name='verbose',help='Display extra information on STDOUT'"`
}

type Config struct {
	Logo struct {
		Filename      string `yaml:"filename"`
		VerticalSpace int    `yaml:"vertical-space"`
	} `yaml:"logo"`

	Text struct {
		Salutation string `yaml:"salutation"`
		Signature  string `yaml:"signature"`
	} `yaml:"text"`
}

type Secrets struct {
	API struct {
		Token    string `yaml:"token"`
		ShopName string `yaml:"shop"`
	} `yaml:"api"`
}

type AllConfig struct {
	Config  Config
	Secrets Secrets
}

// myPdf embeds gopdf.GoPdf so I can create a WriteLine method later
// https://stackoverflow.com/questions/28800672/how-to-add-new-methods-to-an-existing-type-in-go
type myPdf struct {
	*gopdf.GoPdf
}

const pageWidth = 144  // points
const pageHeight = 504 // points
const lineSpacing = 13
const fontSize = 10

// createPDF sets up a gopdf.GoPdf document for the packing slip label
func createPDF() (*myPdf, error) {
	pdf := &myPdf{&gopdf.GoPdf{}}
	labelSize := &gopdf.Rect{W: pageWidth, H: pageHeight}
	pdf.Start(gopdf.Config{PageSize: *labelSize})
	pdf.AddPage()

	if err := pdf.AddTTFFont("regular", "fonts/arialrounded.ttf"); err != nil {
		return nil, err
	}

	if err := pdf.AddTTFFont("bold", "fonts/arialroundedbold.ttf"); err != nil {
		return nil, err
	}

	if err := pdf.SetFont("regular", "", fontSize); err != nil {
		return nil, err
	}

	return pdf, nil
}

// writeLine writes a line to the PDF.
// It wraps long strings at based on pageWidth-rightMargin.
// More than 1 trailing newline characters are converted to additional line breaks.
func (p *myPdf) writeLine(s string) {
	trimmed := strings.TrimRight(s, "\n")
	newlines := len(s) - len(trimmed)

	// if there is any text after trimming the newlines
	// then split it at the pageWidth before writing it to a cell
	if trimmed != "" {
		texts, _ := p.SplitTextWithWordWrap(trimmed, pageWidth-p.MarginRight())
		for _, text := range texts {
			_ = p.Cell(nil, text)
			p.Br(lineSpacing)
		}
	}

	if newlines > 1 {
		p.Br(lineSpacing * float64(newlines-1))
	}
}

// LoadConfig loads the config and secrets yaml files and returns structs
func LoadConfig(configPath, secretsPath string) (*AllConfig, error) {
	// Load plain configuration
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load and decrypt secrets
	secretsData, err := decrypt.File(secretsPath, "yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets file: %w", err)
	}

	var secrets Secrets
	if err := yaml.Unmarshal(secretsData, &secrets); err != nil {
		return nil, fmt.Errorf("failed to parse secrets file: %w", err)
	}

	return &AllConfig{
		Config:  config,
		Secrets: secrets,
	}, nil
}

func main() {
	var cli CLIFlags
	kong.Parse(&cli)

	// usee the default config and secrets file location in ~/.config/packingslipper
	// if those flags aren't specified
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	if cli.ConfigFilename == "" {
		cli.ConfigFilename = filepath.Join(home, ".config", "packingslipper", "configuration.yaml")
	}
	if cli.SecretsFilename == "" {
		cli.SecretsFilename = filepath.Join(home, ".config", "packingslipper", "secrets.enc.yaml")
	}
	if cli.Verbose {
		log.Info("Using config", "configuration", cli.ConfigFilename)
		log.Info("Using config", "secrets", cli.SecretsFilename)
	}

	// load the configuration files
	cfg, err := LoadConfig(cli.ConfigFilename, cli.SecretsFilename)
	if err != nil {
		log.Fatal(err)
	}

	// create the blank label
	p, err := createPDF()
	if err != nil {
		log.Fatal(err)
	}

	// create a new shopify app and api client
	app := goshopify.App{}
	client, err := goshopify.NewClient(app, cfg.Secrets.API.ShopName, cfg.Secrets.API.Token)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orders, err := client.Order.List(ctx, goshopify.OrderListOptions{Status: "any"})
	if err != nil {
		log.Fatal(err)
	}

	// get latest entry
	latest := orders[cli.OrderOffset]
	if cli.Verbose {
		log.Info("Got orders", "latest", latest.Name)
	}

	p.Image(cfg.Config.Logo.Filename, p.MarginLeft(), p.MarginTop(), nil)

	p.SetXY(p.MarginLeft(), float64(cfg.Config.Logo.VerticalSpace))
	p.writeLine("Order " + latest.Name)
	p.writeLine(latest.CreatedAt.Format("Jan 1, 2006") + "\n\n")

	p.SetFont("bold", "", fontSize)
	p.writeLine("SHIP TO\n")

	p.SetFont("regular", "", fontSize)
	p.writeLine(latest.ShippingAddress.FirstName + " " + latest.ShippingAddress.LastName)
	p.writeLine(latest.ShippingAddress.Address1)
	if latest.ShippingAddress.Address2 != "" {
		p.writeLine(latest.ShippingAddress.Address2)
	}

	citystate := strings.Builder{}
	citystate.WriteString(latest.ShippingAddress.City)
	citystate.WriteString(" ")
	citystate.WriteString(latest.ShippingAddress.ProvinceCode)
	citystate.WriteString(" ")
	citystate.WriteString(latest.ShippingAddress.Zip)
	citystate.WriteString("\n")
	p.writeLine(citystate.String())
	p.writeLine(latest.ShippingAddress.Country + "\n\n")

	for _, lineItem := range latest.LineItems {
		p.SetFont("regular", "", fontSize)
		p.writeLine(fmt.Sprintf("Qty %d", lineItem.Quantity))
		p.SetFont("bold", "", fontSize)
		p.writeLine(lineItem.Name)
		p.SetFont("regular", "", fontSize)
		p.writeLine("SKU: " + lineItem.SKU + "\n\n")
	}

	p.writeLine(cfg.Config.Text.Salutation)
	p.SetFont("bold", "", fontSize)
	p.writeLine(cfg.Config.Text.Signature)

	p.WritePdf(cli.OutFilename)
}
