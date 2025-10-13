if test (uname) = Linux
    set -gx JAVA_HOME /usr/lib/jvm/java-latest-openjdk
    if not test -d $JAVA_HOME
        echo "Warning: JAVA_HOME path '$JAVA_HOME' does not exist."
    end
end
