# Peerview Backupinator 3000

Back up google docs files in PDF

## Execution

1. Go to google drive, create a desktop client and get the secret keys. Save it as credentials.json and paste it in the working directory.
2. `go run .`
3. Go to the link, sign in, then copy the token. It should be in this format: `http://localhost/?state=state-token&code=<TOKEN GOES HERE>&scope=https://www.googleapis.com/auth/documents.readonly`
4. Paste it into the command line.

After doing these steps, a classmate's google docs should be downloaded automatically.
