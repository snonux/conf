#!/usr/bin/env ruby

require 'optparse'
require 'digest'
require 'json'
require 'set'

PERSONAL_TIMESPAN_D = 30
WORK_TIMESPAN_D = 14
WORKTIME_DIR = "#{ENV['HOME']}/git/worktime".freeze
GOS_DIR = "#{ENV['HOME']}/.gosdir".freeze
MAX_PENDING_RANDOM_TASKS = 11

def maybe?
  [true, false].sample
end

def run_from_personal_device?
  `uname`.chomp == 'Linux'
end

def random_count
  MAX_PENDING_RANDOM_TASKS - `task status:pending +random count`.to_i
end

def notes(notes_dirs, prefix, dry)
  notes_dirs.each do |notes_dir|
    Dir["#{notes_dir}/#{prefix}-*"].each do |notes_file|
      match = File.read(notes_file).strip.match(/(?<due>\d+)? *(?<tag>[A-Z]?[a-z,-:]+) *(?<body>.*)/m)
      next unless match

      tags = match[:tag].downcase.split(',') + [prefix]
      due = if match[:due].nil?
              tags.include?('track') ? '1year' : "#{rand(0..PERSONAL_TIMESPAN_D)}d"
            else
              "#{match[:due]}d"
            end
      yield tags, match[:body], due
      File.delete(notes_file) unless dry
    end
  end
end

def random_quote(md_file)
  tag = File.basename(md_file, '.md').downcase
  lines = File.readlines(md_file)

  match = lines.first.match(/\((\d+)\)/)
  timespan = run_from_personal_device? ? PERSONAL_TIMESPAN_D : WORK_TIMESPAN_D
  timespan = match ? match[1].to_i : timespan

  quote = lines.select { |l| l.start_with? '*' }.map { |l| l.sub(/\* +/, '') }.sample
  tags = [tag, 'random']
  tags << 'work' if maybe? and maybe?
  yield tags, quote.chomp, "#{rand(0..timespan)}d"
end

def run!(cmd, dry)
  puts cmd
  return if dry

  puts `#{cmd}`
  raise "Command '#{cmd}' failed with #{$?.exitstatus}" if $?.exitstatus != 0
rescue StandardError => e
  puts "Error running command '#{cmd}': #{e.message}"
  exit 1
end

def skill_add!(skills_str, dry)
  skills_file = "#{WORKTIME_DIR}/skills.txt"
  skills_str.split(',').map(&:strip).each { skills[_1.to_s.downcase] = _1 }

  File.foreach(skills_file) do |line|
    line.chomp!
    skills[line.downcase] = line
  end
  File.open("#{skills_file}.tmp", 'w') do |file|
    skills.each_value { |skill| file.puts(skill) }
  end
  return if dry

  File.rename("#{skills_file}.tmp", skills_file)
end

def worklog_add!(tag, quote, due, dry)
  file = "#{WORKTIME_DIR}/wl-#{Time.now.to_i}n.txt"
  content = "#{due.chomp 'd'} #{tag} #{quote}"

  puts "#{file}: #{content}"
  File.write(file, content) unless dry
end

# Queue to Gos https://codeberg.org/snonux/gos
def gos_queue!(tags, message, dry)
  tags.delete('share')
  platforms = []
  %w[linkedin li mastodon ma noop no].select { tags.include?(_1) }.each do |platform|
    platforms << platform
    tags.delete(platform)
  end
  unless platforms.empty?
    platforms = %w[share] + platforms
    tags = ["#{platforms.join(':')}"] + tags
  end
  tags = %w[share] + tags if tags.size == 1 && !tags.first.start_with?('share')
  tags_str = tags.join(',')

  message = "#{tags_str.empty? ? '' : "#{tags_str} "}#{message}"
  file = "#{GOS_DIR}/#{Digest::MD5.hexdigest(message)}.txt"
  puts "Writing #{file} with #{message}"
  File.write(file, message) unless dry
end

def task_add!(tags, quote, due, dry)
  if quote.empty?
    puts 'Not adding task with empty quote'
    return
  end
  if tags.include?('tr')
    tags << 'track'
    tags.delete('tr')
  end
  tags << 'work' if tags.include?('mentoring') || tags.include?('productivity')
  tags.uniq!

  if tags.include?('task')
    run! "task #{quote}", dry
  else
    project = tags.find { |t| t =~ /^[A-Z]/ }
    project = if project.nil?
                ''
              else
                tags.delete!(project)
                " project:#{project.downcase}"
              end
    priority = tags.include?('high') ? 'H' : ''
    run! "task add due:#{due} priority:#{priority}#{project} +#{tags.join(' +')} '#{quote.gsub("'", '"')}'", dry
  end
end

def task_schedule!(id, due, dry)
  run! "timeout 5s task modify #{id} due:#{due}", dry
end

# Randomly schedule all unscheduled tasks but the ones with the +unsched tag
def unscheduled_tasks
  lines = `task -lowhigh -unsched -nosched -notes -note -meeting -track due: 2>/dev/null`.split("\n").drop(1)
  lines.pop
  lines.map { |foo| foo.split.first }.each do |id|
    yield id if id.to_i.positive?
  end
end

begin
  opts = {
    quotes_dir: "#{ENV['HOME']}/Notes/HabitsAndQuotes",
    notes_dirs: "#{ENV['HOME']}/Notes,#{ENV['HOME']}/Notes/Quicklogger,#{ENV['HOME']}/git/worktime",
    dry_run: false,
    no_random: false
  }

  opt_parser = OptionParser.new do |o|
    o.banner = 'Usage: ruby taskwarriorfeeder.rb [options]'
    o.on('-d', '--quotes-dir DIR', 'The quotes directory') { |v| opts[:quotes_dir] = v }
    o.on('-n', '--notes-dirs DIR1,DIR2,...', 'The notes directories') { |v| opts[:notes_dirs] = v }
    o.on('-D', '--dry-run', 'Dry run mode') { opts[:dry_run] = true }
    o.on('-R', '--no-randoms', 'No random entries') { opts[:no_random] = true }
    o.on_tail('-h', '--help', 'Show this help message and exit') { puts o and exit }
  end

  opt_parser.parse!(ARGV)
  core_habits_md_file = "#{opts[:quotes_dir]}/CoreHabits.md"

  (run_from_personal_device? ? %w[ql pl] : %w[wl]).each do |prefix|
    notes(opts[:notes_dirs].split(','), prefix, opts[:dry_run]) do |tags, note, due|
      if tags.include?('skill') || tags.include?('skills')
        skill_add!(note, opts[:dry_run])
      elsif tags.include? 'work'
        worklog_add!(:log, note, due, opts[:dry_run])
      elsif tags.any? { |tag| tag.start_with?('share') }
        gos_queue!(tags, note, opts[:dry_run])
      else
        task_add!(tags, note, due, opts[:dry_run])
      end
    end
  end

  unless opts[:no_random]
    if File.exist?(core_habits_md_file)
      random_quote(core_habits_md_file) do |tags, quote, due|
        task_add!(tags, quote, due, opts[:dry_run])
      end
    end
    count = random_count

    Dir["#{opts[:quotes_dir]}/*.md"].shuffle.each do |md_file|
      next unless maybe?
      break if count <= 0

      random_quote(md_file) do |tags, quote, due|
        task_add!(tags, quote, due, opts[:dry_run])
        count -= 1
      end
    end
  end

  if Dir.exist?(GOS_DIR) && !opts[:dry_run]
    Dir["#{WORKTIME_DIR}/tw-gos-*.json"].each do |tw_gos|
      JSON.parse(File.read(tw_gos)).each do |entry|
        gos_queue!(entry['tags'], entry['description'], opts[:dry_run])
      end
      File.delete(tw_gos)
    rescue StandardError => e
      puts e
    end
  end

  unscheduled_tasks do |id|
    task_schedule!(id, "#{rand(0..PERSONAL_TIMESPAN_D)}d", opts[:dry_run])
  end
end
