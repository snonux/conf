function track::add_record
    set -l value $argv[1]
    set -l description $argv[2]

    set -l timeseries_names \
        "agentic coding (minutes, mental)" \
        "articles (minutes, mental)" \
        "audio books (minutes, mental)" \
        "awesome (effort, mental)" \
        "book notes (minutes, mental)" \
        "breathing (minutes, mental)" \
        "cardio (minutes, health)" \
        "yoga (minutes, health)" \
        "household (minutes, health)" \
        "coding (minutes, mental)" \
        "notes and tinkering (minutes, mental)" \
        "doing nothing (minutes, mental)" \
        "fasting (effort, health)" \
        "foo.zone (minutes, mental)" \
        "good day (effort, mental)" \
        "helix (minutes, mental)" \
        "infra (minutes, mental)" \
        "learning bulgarian (minutes, mental)" \
        "learning tech (minutes, mental)" \
        "low carb day (effort, health)" \
        "meditation (minutes, mental)" \
        "no pod soda (effort, health)" \
        "obstacle like bad sleep (effort, obstacle)" \
        "podcasts (minutes, mental)" \
        "real coding (minutes, mental)" \
        "steps (effort, health)" \
        "stretching (minutes, health)" \
        "journaling (minutes, mental)" \
        "tech books (minutes, mental)" \
        "touch typing (minutes, mental)" \
        abort
    set -l selected (printf '%s\n' $timeseries_names | fzf --prompt="Select time series: ")

    if test -z "$selected"; or test "$selected" = abort
        echo "No record added."
        return
    end

    set -l name (string replace -r ' \([^)]+\)$' '' $selected)
    set -l unit (string match -r '\(([^,]+),' $selected | tail -n1)
    set -l kind (string match -r ', ([^)]+)\)' $selected | tail -n1)

    set -l epoch (date +%s)
    set -l year_week (date +%Y-%V)
    set -l current_date (date +%Y-%m-%d)
    set -l csv_file ~/git/worktime/track-(hostname).csv

    echo "$name,$value,$unit,$kind,$epoch,$year_week,$current_date,$description" >>$csv_file
    echo "Added: $name, $value $unit, $kind, $year_week,$description"
end

function track::report
    set -l now (date +%s)
    set -l day_of_week (date +%u)

    set -l days_since_saturday (math "($day_of_week + 1) % 7")
    set -l last_saturday (math "$now - $days_since_saturday * 24 * 3600")

    for week_offset in (seq 0 3)
        set -l week_start (math "$last_saturday - $week_offset * 7 * 24 * 3600")
        set -l week_end (math "$week_start + 6 * 24 * 3600 + 86399")

        set -l start_date (date -d @$week_start +%Y-%m-%d)
        set -l end_date (date -d @$week_end +%Y-%m-%d)

        set -l mental_minutes 0
        set -l mental_effort 0
        set -l health_minutes 0
        set -l health_effort 0
        set -l obstacle_minutes 0
        set -l obstacle_effort 0

        for csv_file in ~/git/worktime/track-*.csv
            if test -f $csv_file
                while read -l line
                    set -l fields (string split ',' $line)
                    set -l name $fields[1]
                    set -l value $fields[2]
                    set -l unit $fields[3]
                    set -l kind $fields[4]
                    set -l epoch $fields[5]

                    if test $epoch -ge $week_start -a $epoch -le $week_end
                        if test "$name" = steps
                            set value (math $value / 10000)
                        else if test "$name" = fasting
                            set value (math $value / 20)
                        end

                        switch $kind
                            case mental
                                if test "$unit" = minutes
                                    set mental_minutes (math $mental_minutes + $value)
                                else if test "$unit" = effort
                                    set mental_effort (math $mental_effort + $value)
                                end
                            case health
                                if test "$unit" = minutes
                                    set health_minutes (math $health_minutes + $value)
                                else if test "$unit" = effort
                                    set health_effort (math $health_effort + $value)
                                end
                            case obstacle
                                if test "$unit" = minutes
                                    set obstacle_minutes (math $obstacle_minutes + $value)
                                else if test "$unit" = effort
                                    set obstacle_effort (math $obstacle_effort + $value)
                                end
                        end
                    end
                end <$csv_file
            end
        end

        set -l mental_hours (math $mental_minutes / 60)
        set -l health_hours (math $health_minutes / 60)
        set -l obstacle_hours (math $obstacle_minutes / 60)

        echo ""
        echo "Week: $start_date to $end_date"
        printf "  Kind     | Hours   | Effort\n"
        printf "  ---------|---------|-------\n"
        printf "  mental   | %7.2f | %6.2f\n" $mental_hours $mental_effort
        printf "  health   | %7.2f | %6.2f\n" $health_hours $health_effort
        printf "  obstacle | %7.2f | %6.2f\n" $obstacle_hours $obstacle_effort
    end
end

function track::today
    set -l today (date +%Y-%m-%d)
    echo "Entries for $today:"
    for csv_file in ~/git/worktime/track-*.csv
        if test -f $csv_file
            while read -l line
                set -l fields (string split ',' $line)
                set -l name $fields[1]
                set -l value $fields[2]
                set -l unit $fields[3]
                set -l kind $fields[4]
                set -l date $fields[7]
                set -l description $fields[8]

                if test "$date" = "$today"
                    printf "  %-30s | %6s %-7s | %-8s" $name $value $unit $kind
                    if test -n "$description"
                        printf " | %s" $description
                    end
                    printf "\n"
                end
            end <$csv_file
        end
    end
end

function track::weekly
    set -l now (date +%s)
    set -l day_of_week (date +%u)
    
    set -l days_since_saturday (math "($day_of_week + 1) % 7")
    set -l week_start (math "$now - $days_since_saturday * 24 * 3600")
    set -l week_end (math "$week_start + 6 * 24 * 3600 + 86399")
    
    set -l start_date (date -d @$week_start +%Y-%m-%d)
    set -l end_date (date -d @$week_end +%Y-%m-%d)
    
    echo "Current Week: $start_date to $end_date"
    echo ""
    
    set -l activities
    
    for csv_file in ~/git/worktime/track-*.csv
        if test -f $csv_file
            while read -l line
                set -l fields (string split ',' $line)
                set -l name $fields[1]
                set -l value $fields[2]
                set -l unit $fields[3]
                set -l kind $fields[4]
                set -l epoch $fields[5]
                
                if test $epoch -ge $week_start -a $epoch -le $week_end
                    set -l found 0
                    
                    for i in (seq 1 (count $activities))
                        set -l activity $activities[$i]
                        set -l activity_fields (string split '|' $activity)
                        if test "$name" = "$activity_fields[1]"
                            set -l new_value (math $activity_fields[2] + $value)
                            set activities[$i] "$name|$new_value|$unit|$kind"
                            set found 1
                            break
                        end
                    end
                    
                    if test $found -eq 0
                        set -a activities "$name|$value|$unit|$kind"
                    end
                end
            end <$csv_file
        end
    end
    
    for kind in mental health obstacle
        set -l kind_activities
        for activity in $activities
            set -l fields (string split '|' $activity)
            if test "$fields[4]" = "$kind"
                set -a kind_activities $activity
            end
        end
        
        if test (count $kind_activities) -gt 0
            echo "=== $kind ==="
            
            set -l sorted (printf '%s\n' $kind_activities | while read -l activity
                set -l fields (string split '|' $activity)
                printf "%010.2f|%s\n" $fields[2] $activity
            end | sort -rn | string replace -r '^[^|]+\|' '')
            
            for activity in $sorted
                set -l fields (string split '|' $activity)
                set -l name $fields[1]
                set -l value $fields[2]
                set -l unit $fields[3]
                
                if test "$unit" = minutes
                    set -l hours (math $value / 60)
                    printf "  %-35s %8.2f hours (%6.0f min)\n" $name $hours $value
                else
                    printf "  %-35s %8.2f %s\n" $name $value $unit
                end
            end
            echo ""
        end
    end
end

function track::edit
    set -l csv_file ~/git/worktime/track-(hostname).csv
    $EDITOR $csv_file
end

abbr -a tra track::add_record
abbr -a treport track::report
abbr -a troday track::today
abbr -a tredit track::edit
abbr -a treekly track::weekly
