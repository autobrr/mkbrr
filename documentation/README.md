# mkbrr.com

This folder holds the source of the [mkbrr.com](https://mkbrr.com) docs site. Mintlify builds the site from the `main` branch, so the live site describes the latest release. See `docs/adr/0002-develop-main-and-docs-site.md`.

## Preview

Install the [Mintlify CLI](https://www.npmjs.com/package/mintlify):

```
npm i -g mintlify
```

Run this command in this folder, where `docs.json` is:

```
mintlify dev
```

If `mintlify dev` does not start, run `mintlify install` to install its dependencies again.
