PATH=$PATH:/usr/local/bin

# These .task.status files were synced by my Laptops to the server.
find /home -maxdepth 2 -mindepth 2 -type f -name .task.status \
    | while read file; do
    echo $file:
    cat $file
    echo
    stat $file
    echo
done
