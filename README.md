# PackingSlipper

PackingSlipper is a command-line tool for creating a PDF packing slip from a Shopify order.

You probably want to use
[Shopify's instructions for creating custom CSS for your packing slips](https://help.shopify.com/en/manual/fulfillment/managing-orders/printing-orders/packing-slips/customizing-packing-slips)
instead of using this. I used my own custom CSS for years so that I could print packing slips on 2x7 Dymo labels
(which I have way too many of since I started using a 4x6 label printer), but it seems to have started creating
page breaks in the PDFs that it creates. So I made this. It creates a single packing slip based on the most recent
order, by default.

## Installing

1. [Get Go](https://go.dev/doc/install)
2. Run `go install github.com/rahji/packingslipper@latest`

*I would normally create binaries so you can download and run it without installing Go, but I don't expect anyone
to use this. And if they do, they'll likely want to change something in the code. This was really made just to
solve my specific problem*

## Configuration

You'll need to create a [Shopify Custom App](https://help.shopify.com/en/manual/apps/app-types#custom-apps) for your
store and give it permission to view orders. Note the API token that you are given.

### Secrets

The secrets configuration for this project is a bit overkill, but the whole thing is really a learning exercise anyway.
SOPS is a pretty simple tool for encrypting files. Age is a modern PGP alternative for doing the actual encryption. And
the ~~go.mozilla.org/sops/v3/decrypt~~ **github.com/getsops/sops/v3** package (never use that mozilla one!) provides an easy way to use those encrypted files inside a Go program.

The secrets for this program are the token from above and your Shopify shop name (as shown at the beginning of your Shopify admin site URL).
These secrets are stored in an encrypted YAML file. Create the YAML file and encrypt it using SOPS. You'll probably
need to [install SOPS](https://getsops.io/) first. And if you want to use Age for the encryption, as I did, you'll need to
[install Age](https://github.com/FiloSottile/age) and generate a key pair:

```bash
mkdir -p ~/.config/sops/age
age-keygen -o ~/.config/sops/age/keys.txt
chmod 700 ~/.config/sops/age/keys.txt
````

Note: If you store your key file in a non-standard directory then you'll want to set an environment variable with its
location: `export SOPS_AGE_KEY_FILE=/path/to/your/age-key.txt`

Create a `.sops.yaml` file, so SOPS knows what your Age public key is. (It was shown to you by `age-keygen`.)

```yaml
creation_rules:
  - age: >-
      YOUR_PUBLIC_AGE_KEY
```

Create an unencrypted file called `secrets.enc.yaml`:

```yaml
api:
  token: "shpat_..."
  shop: "your-shop-name"
```

Then encrypt it, in place: `sops -e -i secrets.enc.yaml`

### Other Config

Edit the included `configuration.yaml` file, according to your needs.

## Usage

Open your terminal application and type `packingslipper`

The program accepts these flags:

| Flag | Default | Description |
| ---- | ------- | ----------- |
| outfile | packingslip.pdf | Output PDF filename |
| offset | 0 | How far back to jump from the most recent order |
| config | configuration.yaml | Configuration YAML filename (default: ~/.config/packingslipper/configuration.yaml) |
| secrets | secrets.enc.yaml | Encrypted secrets YAML filename (default: ~/.config/packingslipper/secrets.enc.yaml) |
| verbose | false | Display extra information on STDOUT |

## Issues

Because of the font that I am using, addresses with characters from other languages are not going to work. I tried
fonts that provide those character sets, but then I went down a rathole of having the program make a guess at
the language and substitute the correct font. Unfortunately, it seems as though it's not so easy to correctly
determine whether text is Chinese or Japanese (which seems insane to me).

