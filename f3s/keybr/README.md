# keybr.com

Self-hosted deployment of [keybr.com](https://github.com/aradzie/keybr.com) - a typing tutor.

## Prerequisites

Before deploying, create the persistent volume directory on the k3s node:

```bash
mkdir -p /data/nfs/k3svolumes/keybr/data
```

## Deploy

```bash
just install
```

## Upgrade

```bash
just upgrade
```

## Remove

```bash
just delete
```

## Access

http://keybr.f3s.buetow.org

## Backup Progress (Anonymous Mode)

In anonymous mode, keybr stores your progress in the browser's IndexedDB.

### Option 1: Built-in Export

1. Go to the **Profile** page on keybr
2. Click the **Download** button to export your stats as a file

### Option 2: Manual IndexedDB Export (Firefox)

1. Open keybr in Firefox
2. Press `F12` to open Developer Tools
3. Go to **Storage** tab → **Indexed DB** → expand the site URL
4. Find the `history` database with your results

To export via Console (`F12` → Console):

```javascript
let request = indexedDB.open('history');
request.onsuccess = () => {
  let db = request.result;
  let tx = db.transaction('results', 'readonly');
  let store = tx.objectStore('results');
  let getAll = store.getAll();
  getAll.onsuccess = () => {
    let blob = new Blob([JSON.stringify(getAll.result)], {type: 'application/json'});
    let a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'keybr-backup.json';
    a.click();
  };
};
```
