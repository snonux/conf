function kcompletions
    if command -q -v kubectl >/dev/null
        kubectl completion fish | source
    end
end

# Check if the directory $HOME/.krew exists and update PATH
if test -d $HOME/.krew
    set -x PATH (set -q KREW_ROOT; and echo $KREW_ROOT; or echo $HOME/.krew)/bin $PATH
end

function kpod
    set pattern "."
    if test -n "$argv[1]"
        set pattern "$argv[1]"
    end
    set -gx POD (kubectl get pods | grep "$pattern" | sort -R | head -n 1 | cut -d' ' -f1)
    echo "Pod is $POD"
end

function klogsf
    if test -z "$POD" -o -n "$argv[1]"
        kpod $argv
    end
    kubectl logs -f $POD
end

function klogs
    if test -z "$POD" -o -n "$argv[1]"
        kpod $argv
    end
    kubectl logs $POD
end

function kbash
    if test -z "$POD" -o -n "$argv[1]"
        kpod $argv
    end
    kubectl exec -it $POD -- /bin/bash
end

function kshell
    if test -z "$POD" -o -n "$argv[1]"
        kpod $argv
    end
    kubectl exec -it $POD -- /bin/sh
end

function kdesc
    if test -z "$POD" -o -n "$argv[1]"
        kpod $argv
    end
    kubectl describe pod $POD
end

function kedit
    if test -z "$POD" -o -n "$argv[1]"
        kpod $argv
    end
    kubectl edit pod $POD
end

function k8s::kubectl::config::contexts
    kubectl config get-contexts | sed '1d; /\*/d' | awk '{ print $1 }' | sort
end
alias kcontexts="k8s::kubectl::config::contexts"

function k8s::kubectl::config::use_context
    kubectl config use-context (kubectl config get-contexts | sed '1d; /\*/d' | awk '{ print $1 }' | sort | fzf)
end
alias kcontext="k8s::kubectl::config::use_context"

function k8s::kubectl::config::set_namespace
    kubectl config set-context --current --namespace=(kubectl get ns | sed 1d | awk '{ print $1 }' | sort | fzf)
end
alias knamespace="k8s::kubectl::config::set_namespace"
