# keybr

Self-hosted [keybr.com](https://github.com/aradzie/keybr.com) typing tutor,
`https://keybr.f3s.buetow.org` (LAN: `keybr.f3s.lan.buetow.org`). Namespace
`services`, ArgoCD app `keybr`. Volume `/data/nfs/k3svolumes/keybr/data`
(create once).

```sh
just status | logs [lines] | port-forward [3000] | sync | argocd-status | restart
```

## Backing up progress

In anonymous mode progress lives in the browser's IndexedDB, not on the
server. Export it from the Profile page (Download), or from the browser
console:

```javascript
let request = indexedDB.open('history');
request.onsuccess = () => {
  let db = request.result;
  let tx = db.transaction('results', 'readonly');
  let getAll = tx.objectStore('results').getAll();
  getAll.onsuccess = () => {
    let blob = new Blob([JSON.stringify(getAll.result)], {type: 'application/json'});
    let a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'keybr-backup.json';
    a.click();
  };
};
```
