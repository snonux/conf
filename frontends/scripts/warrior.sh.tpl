PATH=$PATH:/usr/local/bin

find /home -maxdepth 2 -mindepth 2 -type f -name .task.status \
    | while read file; do
    echo $file:
    cat $file
done
