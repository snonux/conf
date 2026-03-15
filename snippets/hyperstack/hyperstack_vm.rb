#!/usr/bin/env ruby
# frozen_string_literal: true

begin
  require 'bundler/setup'
rescue LoadError
  nil
end

require 'json'
require 'net/http'
require 'open3'
require 'optparse'
require 'ipaddr'
require 'shellwords'
require 'socket'
require 'time'
require 'timeout'

begin
  require 'toml-rb'
rescue LoadError
  warn "Missing dependency: toml-rb. Run `bundle install` in #{__dir__} first."
  exit 2
end

module HyperstackVM
  class Error < StandardError; end

  class Config
    DEFAULTS = {
      'auth' => {
        'api_key_file' => '~/.hyperstack'
      },
      'hyperstack' => {
        'base_url' => 'https://infrahub-api.nexgencloud.com/v1'
      },
      'state' => {
        'file' => '.hyperstack-vm-state.json'
      },
      'vm' => {
        'name_prefix' => 'gpt-oss',
        'hostname' => 'hyperstack',
        'flavor_name' => 'n3-A100x1',
        'image_name' => 'Ubuntu Server 24.04 LTS R570 CUDA 12.8 with Docker',
        'assign_floating_ip' => true,
        'create_bootable_volume' => false,
        'enable_port_randomization' => false,
        'labels' => %w[gpt-oss-120b wireguard]
      },
      'ssh' => {
        'username' => 'ubuntu',
        'private_key_path' => '~/.ssh/id_rsa',
        'hyperstack_key_name' => 'earth',
        'port' => 22,
        'connect_timeout_sec' => 10
      },
      'network' => {
        'wireguard_udp_port' => 56_710,
        'wireguard_subnet' => '192.168.3.0/24',
        'ollama_port' => 11_434,
        'allowed_ssh_cidrs' => ['0.0.0.0/0'],
        'allowed_wireguard_cidrs' => ['0.0.0.0/0']
      },
      'bootstrap' => {
        'enable_guest_bootstrap' => true,
        'install_wireguard' => true,
        'configure_ufw' => true,
        'configure_ollama_host' => false
      },
      'ollama' => {
        'install' => true,
        'models_dir' => '/ephemeral/ollama/models',
        'listen_host' => '0.0.0.0:11434',
        'gpu_overhead_mb' => 2000,
        'num_parallel' => 4,
        'pull_models' => ['qwen3-coder:30b', 'gpt-oss:20b', 'gpt-oss:120b', 'nemotron-3-super']
      },
      'wireguard' => {
        'auto_setup' => true,
        'setup_script' => './wg1-setup.sh'
      },
      'local_client' => {
        'check_wg1_service' => true,
        'interface_name' => 'wg1',
        'config_path' => '/etc/wireguard/wg1.conf'
      }
    }.freeze

    attr_reader :path

    def self.load(path)
      expanded = File.expand_path(path)
      raise Error, "Config file not found: #{expanded}" unless File.exist?(expanded)

      raw = TomlRB.load_file(expanded)
      new(raw, expanded)
    rescue TomlRB::ParseError => e
      raise Error, "Failed to parse TOML config #{expanded}: #{e.message}"
    end

    def initialize(raw, path)
      @path = path
      @data = deep_merge(DEFAULTS, raw || {})
      validate!
    end

    def api_key
      key_path = expand_path(fetch('auth', 'api_key_file'))
      raise Error, "API key file not found: #{key_path}" unless File.exist?(key_path)

      token = File.readlines(key_path, chomp: true).find { |line| !line.strip.empty? }&.strip
      raise Error, "API key file is empty: #{key_path}" if token.nil? || token.empty?

      token
    rescue Errno::EACCES => e
      raise Error, "Cannot read API key file #{key_path}: #{e.message}"
    end

    def api_base_url
      fetch('hyperstack', 'base_url')
    end

    def state_file
      expand_path(fetch('state', 'file'))
    end

    def environment_name
      fetch('vm', 'environment_name')
    end

    def flavor_name
      fetch('vm', 'flavor_name')
    end

    def image_name
      fetch('vm', 'image_name')
    end

    def vm_name_prefix
      fetch('vm', 'name_prefix')
    end

    def generated_vm_name
      "#{vm_name_prefix}-#{Time.now.utc.strftime('%Y%m%d%H%M%S')}"
    end

    def vm_hostname
      value = fetch('vm', 'hostname')
      return nil if blank?(value)

      value.to_s.downcase
    end

    def assign_floating_ip?
      truthy?(fetch('vm', 'assign_floating_ip'))
    end

    def create_bootable_volume?
      truthy?(fetch('vm', 'create_bootable_volume'))
    end

    def enable_port_randomization?
      truthy?(fetch('vm', 'enable_port_randomization'))
    end

    def labels
      Array(fetch('vm', 'labels')).map(&:to_s)
    end

    def user_data
      custom = custom_user_data
      return custom unless custom.nil? || custom.empty?
      return nil if vm_hostname.nil?

      default_hostname_cloud_init
    rescue Errno::ENOENT => e
      raise Error, "User data file not found: #{e.message}"
    rescue Errno::EACCES => e
      raise Error, "Cannot read user data file: #{e.message}"
    end

    def ssh_username
      fetch('ssh', 'username')
    end

    def ssh_private_key_path
      expand_path(fetch('ssh', 'private_key_path'))
    end

    def ssh_key_name
      fetch('ssh', 'hyperstack_key_name')
    end

    def ssh_port
      Integer(fetch('ssh', 'port'))
    end

    def ssh_connect_timeout
      Integer(fetch('ssh', 'connect_timeout_sec'))
    end

    def wireguard_udp_port
      Integer(fetch('network', 'wireguard_udp_port'))
    end

    def wireguard_subnet
      fetch('network', 'wireguard_subnet')
    end

    def ollama_port
      Integer(fetch('network', 'ollama_port'))
    end

    def allowed_ssh_cidrs
      Array(fetch('network', 'allowed_ssh_cidrs')).map(&:to_s)
    end

    def allowed_wireguard_cidrs
      Array(fetch('network', 'allowed_wireguard_cidrs')).map(&:to_s)
    end

    def guest_bootstrap_enabled?
      truthy?(fetch('bootstrap', 'enable_guest_bootstrap'))
    end

    def install_wireguard?
      truthy?(fetch('bootstrap', 'install_wireguard'))
    end

    def configure_ufw?
      truthy?(fetch('bootstrap', 'configure_ufw'))
    end

    def configure_ollama_host?
      truthy?(fetch('bootstrap', 'configure_ollama_host'))
    end

    def ollama_install_enabled?
      truthy?(fetch('ollama', 'install'))
    end

    def ollama_models_dir
      fetch('ollama', 'models_dir')
    end

    def ollama_listen_host
      fetch('ollama', 'listen_host')
    end

    def ollama_gpu_overhead_mb
      Integer(fetch('ollama', 'gpu_overhead_mb'))
    end

    def ollama_num_parallel
      Integer(fetch('ollama', 'num_parallel'))
    end

    def ollama_pull_models
      Array(fetch('ollama', 'pull_models')).map(&:to_s)
    end

    def local_client_checks_enabled?
      truthy?(fetch('local_client', 'check_wg1_service'))
    end

    def local_interface_name
      fetch('local_client', 'interface_name')
    end

    def local_wg_config_path
      fetch('local_client', 'config_path')
    end

    def wireguard_auto_setup?
      truthy?(fetch('wireguard', 'auto_setup'))
    end

    def wireguard_setup_script
      expand_path(fetch('wireguard', 'setup_script'))
    end

    def desired_security_rules
      rules = []

      allowed_ssh_cidrs.each do |cidr|
        rules << firewall_rule('tcp', ssh_port, cidr)
      end

      allowed_wireguard_cidrs.each do |cidr|
        rules << firewall_rule('udp', wireguard_udp_port, cidr)
      end

      rules << firewall_rule('tcp', ollama_port, wireguard_subnet)
      rules.uniq
    end

    private

    def validate!
      %w[auth hyperstack state vm ssh network bootstrap ollama wireguard local_client].each do |section|
        raise Error, "Missing config section [#{section}]" unless @data.key?(section)
      end

      %w[environment_name flavor_name image_name].each do |key|
        raise Error, "Missing [vm].#{key} in config #{path}" if blank?(dig('vm', key))
      end

      if vm_hostname && vm_hostname !~ /\A[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\z/
        raise Error, "Invalid [vm].hostname #{vm_hostname.inspect}; use lowercase letters, digits, and hyphens only."
      end

      %w[username hyperstack_key_name].each do |key|
        raise Error, "Missing [ssh].#{key} in config #{path}" if blank?(dig('ssh', key))
      end

      [wireguard_subnet, *allowed_ssh_cidrs, *allowed_wireguard_cidrs].each do |cidr|
        IPAddr.new(cidr)
      rescue IPAddr::InvalidAddressError => e
        raise Error, "Invalid CIDR #{cidr.inspect}: #{e.message}"
      end
    end

    def firewall_rule(protocol, port, cidr)
      ip = IPAddr.new(cidr)
      {
        'direction' => 'ingress',
        'ethertype' => ip.ipv4? ? 'IPv4' : 'IPv6',
        'protocol' => protocol,
        'port_range_min' => port,
        'port_range_max' => port,
        'remote_ip_prefix' => cidr
      }
    end

    def fetch(section, key)
      dig(section, key)
    end

    def dig(*keys)
      keys.reduce(@data) do |memo, key|
        memo.is_a?(Hash) ? memo[key] : nil
      end
    end

    def blank?(value)
      value.nil? || value.to_s.strip.empty?
    end

    def truthy?(value)
      value == true
    end

    def custom_user_data
      inline = dig('vm', 'user_data')
      return inline unless inline.nil? || inline.empty?

      file = dig('vm', 'user_data_file')
      return nil if file.nil? || file.empty?

      File.read(expand_path(file))
    end

    def default_hostname_cloud_init
      <<~CLOUD_INIT
        #cloud-config
        preserve_hostname: false
        hostname: #{vm_hostname}
      CLOUD_INIT
    end

    def expand_path(value)
      return nil if value.nil?

      string = value.to_s
      return File.expand_path(string) if string.start_with?('~')
      return string if string.start_with?('/')

      File.expand_path(string, File.dirname(path))
    end

    def deep_merge(left, right)
      left.merge(right) do |_key, old_value, new_value|
        if old_value.is_a?(Hash) && new_value.is_a?(Hash)
          deep_merge(old_value, new_value)
        else
          new_value
        end
      end
    end
  end

  class StateStore
    def initialize(path)
      @path = path
    end

    attr_reader :path

    def load
      return nil unless File.exist?(@path)

      JSON.parse(File.read(@path))
    rescue JSON::ParserError => e
      raise Error, "Failed to parse state file #{@path}: #{e.message}"
    end

    def save(payload)
      temp_path = "#{@path}.tmp"
      File.write(temp_path, JSON.pretty_generate(payload))
      File.rename(temp_path, @path)
    end

    def delete
      File.delete(@path) if File.exist?(@path)
    end
  end

  class HyperstackClient
    def initialize(base_url:, api_key:)
      @base_uri = URI(base_url)
      @api_key = api_key
    end

    def list_environments
      response = request(:get, '/core/environments')
      response.fetch('environments', [])
    end

    def list_keypairs
      response = request(:get, '/core/keypairs')
      response.fetch('keypairs', [])
    end

    def list_flavors
      response = request(:get, '/core/flavors')
      Array(response['data']).flat_map do |entry|
        Array(entry['flavors']).map do |flavor|
          flavor.merge(
            'region_name' => flavor['region_name'] || entry['region_name'],
            'gpu' => flavor['gpu'] || entry['gpu']
          )
        end
      end
    end

    def list_images
      response = request(:get, '/core/images')
      Array(response['images']).flat_map do |entry|
        Array(entry['images']).map do |image|
          image.merge(
            'region_name' => image['region_name'] || entry['region_name'],
            'type' => image['type'] || entry['type']
          )
        end
      end
    end

    def list_vms
      response = request(:get, '/core/virtual-machines')
      response.fetch('instances', [])
    end

    def get_vm(vm_id)
      response = request(:get, "/core/virtual-machines/#{vm_id}")
      response.fetch('instance', nil)
    end

    def create_vm(payload)
      request(:post, '/core/virtual-machines', payload)
    end

    def delete_vm(vm_id)
      request(:delete, "/core/virtual-machines/#{vm_id}")
    end

    def create_vm_rule(vm_id, payload)
      request(:post, "/core/virtual-machines/#{vm_id}/sg-rules", payload)
    end

    private

    def request(method, path, payload = nil)
      uri = @base_uri.dup
      uri.path = "#{@base_uri.path}#{path}"

      request = case method
                when :get
                  Net::HTTP::Get.new(uri)
                when :post
                  Net::HTTP::Post.new(uri)
                when :delete
                  Net::HTTP::Delete.new(uri)
                else
                  raise Error, "Unsupported HTTP method: #{method}"
                end

      request['accept'] = 'application/json'
      request['api_key'] = @api_key
      if payload
        request['content-type'] = 'application/json'
        request.body = JSON.generate(payload)
      end

      response = Net::HTTP.start(
        uri.host,
        uri.port,
        use_ssl: uri.scheme == 'https',
        open_timeout: 30,
        read_timeout: 120
      ) { |http| http.request(request) }

      parse_response(response)
    rescue Timeout::Error, Errno::ECONNREFUSED, SocketError, OpenSSL::SSL::SSLError => e
      raise Error, "Hyperstack API request failed for #{path}: #{e.message}"
    end

    def parse_response(response)
      body = response.body.to_s
      payload = body.empty? ? {} : JSON.parse(body)

      if response.code.to_i >= 400 || payload['status'] == false
        message = payload['message'] || payload['error_reason'] || response.message
        raise Error, "Hyperstack API error (HTTP #{response.code}): #{message}"
      end

      payload
    rescue JSON::ParserError => e
      raise Error, "Failed to parse Hyperstack API response: #{e.message}"
    end
  end

  class LocalWireGuard
    def initialize(interface_name:, config_path:)
      @interface_name = interface_name
      @config_path = config_path
    end

    def status
      {
        'service_state' => service_state,
        'config_path' => @config_path,
        'endpoint' => configured_endpoint,
        'config_readable' => !config_contents.nil?
      }
    end

    private

    def service_state
      stdout, _stderr, status = Open3.capture3('systemctl', 'is-active', "wg-quick@#{@interface_name}")
      value = stdout.to_s.strip
      return value unless value.empty?
      return 'active' if status.success?

      'unknown'
    end

    def configured_endpoint
      content = config_contents
      return nil if content.nil?

      parse_wireguard_config(content)['Endpoint']
    end

    def config_contents
      return @config_contents if defined?(@config_contents)

      @config_contents = File.read(@config_path)
    rescue Errno::EACCES, Errno::ENOENT
      stdout, _stderr, status = Open3.capture3('sudo', '-n', 'cat', @config_path)
      @config_contents = status.success? ? stdout : nil
    end

    def parse_wireguard_config(content)
      current_section = nil
      peer = {}

      content.each_line do |line|
        stripped = line.strip
        next if stripped.empty? || stripped.start_with?('#')

        if stripped.start_with?('[') && stripped.end_with?(']')
          current_section = stripped[1..-2]
          next
        end

        key, value = stripped.split('=', 2).map { |part| part&.strip }
        next unless current_section == 'Peer' && key && value

        peer[key] = value
      end

      peer
    end
  end

  class Manager
    def initialize(config:, client:, state_store:, local_wireguard:, out: $stdout)
      @config = config
      @client = client
      @state_store = state_store
      @local_wireguard = local_wireguard
      @out = out
    end

    def create(replace: false, dry_run: false)
      existing_state = @state_store.load
      if existing_state && existing_state['vm_id']
        if replace
          if dry_run
            info "DRY RUN: would delete tracked VM #{existing_state['vm_id']} before creating a replacement."
          else
            delete(vm_id: existing_state['vm_id'], preserve_state_on_failure: true)
          end
        elsif resumable_state?(existing_state)
          if dry_run
            print_resume_dry_run(existing_state)
            return
          end

          info "Resuming tracked VM #{existing_state['vm_id']} provisioning..."
          continue_create(existing_state)
          return
        else
          raise Error,
                "State file #{@state_store.path} already tracks VM #{existing_state['vm_id']}. Use --replace or delete first."
        end
      end

      resolved = resolve_dependencies
      vm_name = @config.generated_vm_name
      if dry_run
        info "Planning VM #{vm_name} in #{resolved[:environment]['name']} using #{@config.flavor_name}..."
      else
        info "Creating VM #{vm_name} in #{resolved[:environment]['name']} using #{@config.flavor_name}..."
      end

      payload = build_create_payload(vm_name, resolved)
      if dry_run
        print_create_dry_run(vm_name, resolved, payload)
        return
      end

      response = @client.create_vm(payload)
      instance = Array(response['instances']).first
      raise Error, 'Hyperstack create response did not include an instance ID.' unless instance && instance['id']

      state = {
        'vm_id' => instance['id'],
        'vm_name' => vm_name,
        'environment_name' => resolved[:environment]['name'],
        'region' => resolved[:environment]['region'],
        'flavor_name' => resolved[:flavor]['name'],
        'image_name' => resolved[:image]['name'],
        'key_name' => resolved[:keypair]['name'],
        'public_ip' => instance['floating_ip'],
        'created_at' => Time.now.utc.iso8601
      }
      @state_store.save(state)
      continue_create(state)
    end

    def delete(vm_id: nil, preserve_state_on_failure: false, dry_run: false)
      state = @state_store.load
      target_vm_id = vm_id || state&.dig('vm_id')
      raise Error, "No VM ID provided and no state file found at #{@state_store.path}." if target_vm_id.nil?

      if dry_run
        print_delete_dry_run(target_vm_id, state, preserve_state_on_failure: preserve_state_on_failure)
        return
      end

      info "Deleting VM #{target_vm_id}..."
      @client.delete_vm(target_vm_id)
      wait_for_deletion(target_vm_id)
      @state_store.delete unless preserve_state_on_failure
      info "VM #{target_vm_id} deleted."
    rescue Error
      raise if preserve_state_on_failure

      @state_store.delete
      raise
    end

    def status
      state = @state_store.load
      if state.nil?
        info "No tracked VM state file at #{@state_store.path}."
      else
        begin
          vm = @client.get_vm(state['vm_id'])
          desired = @config.desired_security_rules.map { |rule| normalize_rule(rule) }
          current = Array(vm['security_rules']).map { |rule| normalize_rule(rule) }
          missing_rules = desired - current

          info "Tracked VM: #{state['vm_id']} #{vm['name']}"
          info "Status: #{vm['status']} / #{vm['vm_state']}"
          info "Public IP: #{connect_host_for(vm) || 'none'}"
          info "Missing firewall rules: #{missing_rules.empty? ? 'none' : missing_rules.size}"
        rescue Error => e
          warn "Unable to load VM #{state['vm_id']}: #{e.message}"
        end
      end

      print_local_wireguard_summary(state&.dig('public_ip'))
    end

    private

    def resumable_state?(state)
      state['vm_id'] && (state['bootstrapped_at'].nil? || ollama_setup_needed?(state) || wireguard_setup_needed?(state))
    end

    def continue_create(state)
      vm_id = state['vm_id']

      vm = wait_for_vm_ready(vm_id)
      ensure_security_rules(vm)
      vm = wait_for_connect_ip(vm_id)
      state['public_ip'] = connect_host_for(vm)
      state['security_rules'] = Array(vm['security_rules']).map { |rule| normalize_rule(rule) }
      @state_store.save(state)

      wait_for_ssh(state['public_ip'])
      if @config.guest_bootstrap_enabled? && state['bootstrapped_at'].nil?
        bootstrap_guest(state['public_ip'])
        state['bootstrapped_at'] = Time.now.utc.iso8601
        @state_store.save(state)
      end

      if ollama_setup_needed?(state)
        setup_ollama(state['public_ip'])
        state['ollama_setup_at'] = Time.now.utc.iso8601
        state['ollama_models_dir'] = @config.ollama_models_dir
        state['ollama_pulled_models'] = desired_ollama_models
        @state_store.save(state)
      end

      if wireguard_setup_needed?(state)
        run_wireguard_setup(state['public_ip'])
        state['wireguard_setup_at'] = Time.now.utc.iso8601
        @state_store.save(state)
      end

      vm = @client.get_vm(vm_id)
      state['security_rules'] = Array(vm['security_rules']).map { |rule| normalize_rule(rule) }
      state['status'] = vm['status']
      state['vm_state'] = vm['vm_state']
      state['provisioned_at'] = Time.now.utc.iso8601
      @state_store.save(state)

      info "VM ready: #{state['public_ip']} (id=#{state['vm_id']})"
      print_local_wireguard_summary(state['public_ip'])
    end

    def build_create_payload(vm_name, resolved)
      payload = {
        'name' => vm_name,
        'count' => 1,
        'environment_name' => resolved[:environment]['name'],
        'flavor_name' => resolved[:flavor]['name'],
        'image_name' => resolved[:image]['name'],
        'key_name' => resolved[:keypair]['name'],
        'assign_floating_ip' => @config.assign_floating_ip?,
        'create_bootable_volume' => @config.create_bootable_volume?,
        'enable_port_randomization' => @config.enable_port_randomization?,
        'security_rules' => @config.desired_security_rules
      }
      payload['labels'] = @config.labels unless @config.labels.empty?
      payload['user_data'] = @config.user_data if @config.user_data
      payload
    end

    def resolve_dependencies
      environment = @client.list_environments.find { |item| item['name'] == @config.environment_name }
      raise Error, "Environment #{@config.environment_name.inspect} was not found in Hyperstack." unless environment

      flavor = @client.list_flavors.find do |item|
        item['name'] == @config.flavor_name && item['region_name'] == environment['region']
      end
      raise Error, "Flavor #{@config.flavor_name.inspect} is not available in #{environment['region']}." unless flavor

      if flavor['stock_available'] == false
        raise Error,
              "Flavor #{@config.flavor_name.inspect} exists in #{environment['region']} but is out of stock."
      end

      image = @client.list_images.find do |item|
        item['name'] == @config.image_name && item['region_name'] == environment['region']
      end
      raise Error, "Image #{@config.image_name.inspect} is not available in #{environment['region']}." unless image

      keypair = @client.list_keypairs.find do |item|
        item['name'] == @config.ssh_key_name && item.dig('environment', 'name') == environment['name']
      end
      unless keypair
        raise Error,
              "Keypair #{@config.ssh_key_name.inspect} was not found in environment #{environment['name']}."
      end

      {
        environment: environment,
        flavor: flavor,
        image: image,
        keypair: keypair
      }
    end

    def wait_for_vm_ready(vm_id)
      with_polling("VM #{vm_id} to become ready for firewall updates") do
        vm = @client.get_vm(vm_id)
        next nil if vm.nil?

        raise Error, "VM #{vm_id} entered failed state #{vm['status']} / #{vm['vm_state']}." if failed_vm?(vm)

        vm_ready_for_updates?(vm) ? vm : nil
      end
    end

    def wait_for_connect_ip(vm_id)
      ip_label = @config.assign_floating_ip? ? 'floating IP' : 'reachable IP'
      with_polling("VM #{vm_id} to receive a #{ip_label}") do
        vm = @client.get_vm(vm_id)
        raise Error, "VM #{vm_id} entered failed state #{vm['status']} / #{vm['vm_state']}." if failed_vm?(vm)

        connect_host_for(vm) ? vm : nil
      end
    end

    def wait_for_ssh(host)
      info "Waiting for SSH on #{host}:#{@config.ssh_port}..."
      with_polling("SSH on #{host}:#{@config.ssh_port}") do
        next nil unless tcp_open?(host, @config.ssh_port)

        stdout, stderr, status = run_ssh_command(host, 'true')
        if status.success?
          true
        else
          warn "SSH not ready yet: #{stderr.strip}" unless stderr.to_s.strip.empty?
          nil
        end
      end
    end

    def ensure_security_rules(vm)
      existing = Array(vm['security_rules']).map { |rule| normalize_rule(rule) }
      desired = @config.desired_security_rules.map { |rule| normalize_rule(rule) }

      (desired - existing).each do |rule|
        info "Adding Hyperstack firewall rule #{rule['protocol']} #{rule['remote_ip_prefix']} #{rule['port_range_min']}..."
        @client.create_vm_rule(vm['id'], rule)
      end
    end

    def bootstrap_guest(host)
      info 'Bootstrapping Ubuntu guest over SSH...'
      stdout, stderr, status = run_ssh_command(host, guest_bootstrap_script)
      raise Error, "Guest bootstrap failed: #{stderr.strip.empty? ? stdout : stderr}" unless status.success?
    end

    def ollama_setup_needed?(state)
      return false unless @config.ollama_install_enabled?

      state['ollama_setup_at'].nil? || model_list_signature(desired_ollama_models) != model_list_signature(state['ollama_pulled_models'])
    end

    def setup_ollama(host)
      info "Installing and configuring Ollama on #{host}..."
      output, status = run_ssh_command_streaming(host, ollama_setup_script)
      return if status.success?

      raise Error, "Ollama setup failed: #{output.strip}"
    end

    def wireguard_setup_needed?(state)
      return false unless @config.wireguard_auto_setup?

      public_ip = state['public_ip'].to_s.strip
      return true if public_ip.empty?

      expected_endpoint = "#{public_ip}:#{@config.wireguard_udp_port}"
      @local_wireguard.status['endpoint'] != expected_endpoint
    end

    def run_wireguard_setup(host)
      validate_wireguard_setup_script!
      info "Running WireGuard auto-setup via #{@config.wireguard_setup_script} #{host}..."

      Open3.popen2e('bash', @config.wireguard_setup_script, host) do |stdin, output, wait_thr|
        stdin.sync = true
        stdin.puts
        stdin.close

        output.each do |line|
          @out.print(line)
        end

        status = wait_thr.value
        next if status.success?

        raise Error, "WireGuard setup script failed with exit status #{status.exitstatus}."
      end
    end

    def wait_for_deletion(vm_id)
      info "Waiting for VM #{vm_id} deletion to complete..."
      with_polling("VM #{vm_id} deletion", timeout: 300) do
        @client.get_vm(vm_id)
        nil
      rescue Error => e
        raise unless e.message.include?('not_found') || e.message.include?('does not exists')

        true
      end
    end

    def connect_host_for(vm)
      return vm['floating_ip'] if @config.assign_floating_ip?

      vm['floating_ip'] || vm['fixed_ip']
    end

    def validate_wireguard_setup_script!
      script_path = @config.wireguard_setup_script
      raise Error, "WireGuard setup script not found: #{script_path}" unless File.exist?(script_path)

      mismatches = []
      mismatches << "ssh.username must be 'ubuntu'" unless @config.ssh_username == 'ubuntu'
      mismatches << "local_client.interface_name must be 'wg1'" unless @config.local_interface_name == 'wg1'
      mismatches << 'network.wireguard_udp_port must be 56710' unless @config.wireguard_udp_port == 56_710
      mismatches << "network.wireguard_subnet must be '192.168.3.0/24'" unless @config.wireguard_subnet == '192.168.3.0/24'

      return if mismatches.empty?

      raise Error, "Configured WireGuard settings do not match #{script_path}: #{mismatches.join('; ')}"
    end

    def failed_vm?(vm)
      [vm['status'], vm['vm_state'], vm['power_state']].compact.any? do |value|
        value.to_s.downcase.match?(/error|failed|deleted|shelved/)
      end
    end

    def vm_ready_for_updates?(vm)
      %w[ACTIVE SHUTOFF HIBERNATED].include?(vm['status'].to_s.upcase)
    end

    def tcp_open?(host, port)
      Socket.tcp(host, port, connect_timeout: @config.ssh_connect_timeout) do |sock|
        sock.close
        true
      end
    rescue Errno::ECONNREFUSED, Errno::ETIMEDOUT, SocketError, IOError
      false
    end

    def run_ssh_command(host, remote_script)
      Open3.capture3(*ssh_command(host), stdin_data: remote_script)
    end

    def run_ssh_command_streaming(host, remote_script)
      combined_output = +''
      Open3.popen2e(*ssh_command(host)) do |stdin, output, wait_thr|
        stdin.write(remote_script)
        stdin.close

        output.each do |line|
          combined_output << line
          @out.print(line)
        end

        return [combined_output, wait_thr.value]
      end
    end

    def ssh_command(host)
      command = [
        'ssh',
        '-o', 'BatchMode=yes',
        '-o', 'StrictHostKeyChecking=accept-new',
        '-o', "ConnectTimeout=#{@config.ssh_connect_timeout}",
        '-p', @config.ssh_port.to_s
      ]
      if File.exist?(@config.ssh_private_key_path)
        command.concat(['-i', @config.ssh_private_key_path])
      else
        warn "SSH private key #{@config.ssh_private_key_path} does not exist; falling back to default ssh-agent identity."
      end

      command << "#{@config.ssh_username}@#{host}"
      command << 'bash -se'
      command
    end

    def with_polling(description, timeout: 900, interval: 5)
      deadline = Time.now + timeout
      loop do
        result = yield
        return result if result

        raise Error, "Timed out waiting for #{description}." if Time.now >= deadline

        sleep interval
      end
    end

    def normalize_rule(rule)
      {
        'direction' => rule['direction'].to_s.downcase,
        'ethertype' => rule['ethertype'].to_s,
        'protocol' => rule['protocol'].to_s.downcase,
        'port_range_min' => integer_or_nil(rule['port_range_min']),
        'port_range_max' => integer_or_nil(rule['port_range_max']),
        'remote_ip_prefix' => rule['remote_ip_prefix'].to_s
      }
    end

    def print_create_dry_run(vm_name, resolved, payload)
      info 'DRY RUN: no VM or state file will be created.'
      info "State file: #{@state_store.path}"
      info "Resolved environment: #{resolved[:environment]['name']} (region #{resolved[:environment]['region']})"
      info "Resolved flavor: #{format_flavor(resolved[:flavor])}"
      info "Resolved image: #{resolved[:image]['name']}"
      info "Resolved SSH keypair: #{resolved[:keypair]['name']}"
      info "Planned VM name: #{vm_name}"
      info 'Create payload:'
      @out.puts(JSON.pretty_generate(payload))
      if @config.guest_bootstrap_enabled?
        info 'Guest bootstrap script:'
        @out.puts(guest_bootstrap_script)
      else
        info 'Guest bootstrap is disabled in config.'
      end
      if @config.ollama_install_enabled?
        info "Ollama will be installed with models stored under #{@config.ollama_models_dir}"
        unless desired_ollama_models.empty?
          info "Ollama models to pre-pull: #{desired_ollama_models.join(', ')}"
        end
      end
      if @config.wireguard_auto_setup?
        info "WireGuard auto-setup script: #{@config.wireguard_setup_script} <vm_public_ip>"
      end
      print_local_wireguard_summary(nil)
    end

    def print_resume_dry_run(state)
      info "DRY RUN: would resume provisioning tracked VM #{state['vm_id']}."
      begin
        vm = @client.get_vm(state['vm_id'])
        info "Tracked VM status: #{vm['status']} / #{vm['vm_state']}"
        info "Tracked VM public IP: #{connect_host_for(vm) || 'none'}"
      rescue Error => e
        warn "Unable to inspect tracked VM #{state['vm_id']}: #{e.message}"
      end
      if @config.guest_bootstrap_enabled?
        info 'Guest bootstrap script:'
        @out.puts(guest_bootstrap_script)
      end
      if ollama_setup_needed?(state)
        info "Ollama would be installed with models stored under #{@config.ollama_models_dir}"
        unless desired_ollama_models.empty?
          info "Ollama models to pre-pull: #{desired_ollama_models.join(', ')}"
        end
      end
      if wireguard_setup_needed?(state)
        info "WireGuard auto-setup script would run: #{@config.wireguard_setup_script} #{state['public_ip'] || '<pending-public-ip>'}"
      end
      print_local_wireguard_summary(state['public_ip'])
    end

    def print_delete_dry_run(target_vm_id, state, preserve_state_on_failure:)
      info 'DRY RUN: no VM will be deleted.'
      begin
        vm = @client.get_vm(target_vm_id)
        info "Delete target: #{target_vm_id} #{vm['name']} (#{vm['status']} / #{vm['vm_state']})"
        info "Delete target public IP: #{connect_host_for(vm) || 'none'}"
      rescue Error => e
        warn "Unable to inspect VM #{target_vm_id} before delete: #{e.message}"
      end

      if state && state['vm_id'].to_i == target_vm_id.to_i
        action = preserve_state_on_failure ? 'would remain unchanged' : 'would be removed'
        info "Tracked state file #{@state_store.path} #{action}."
      else
        info 'No tracked state entry would be modified.'
      end
    end

    def format_flavor(flavor)
      gpu = flavor['gpu'].to_s.empty? ? 'CPU-only' : flavor['gpu']
      [
        flavor['name'],
        gpu,
        "#{flavor['gpu_count']} GPU",
        "#{flavor['ram']} GB RAM",
        "#{flavor['cpu']} vCPU",
        "stock=#{flavor['stock_available']}"
      ].join(', ')
    end

    def guest_bootstrap_script
      script = []
      script << 'set -euo pipefail'

      if @config.install_wireguard?
        script << 'which wg >/dev/null 2>&1 || (sudo apt-get update && sudo apt-get install -y wireguard)'
      end

      if @config.configure_ufw?
        script << "sudo ufw allow #{@config.ssh_port}/tcp comment 'Allow SSH' >/dev/null 2>&1 || true"
        script << 'sudo ufw --force enable >/dev/null 2>&1 || true'
        script << "sudo ufw allow #{@config.wireguard_udp_port}/udp comment 'WireGuard #{@config.local_interface_name}' >/dev/null 2>&1 || true"
        script << "sudo ufw allow from #{Shellwords.escape(@config.wireguard_subnet)} to any port #{@config.ollama_port} proto tcp comment 'Ollama via #{@config.local_interface_name}' >/dev/null 2>&1 || true"
      end

      if @config.configure_ollama_host?
        script << "if systemctl list-unit-files | grep -q '^ollama.service'; then"
        script << '  sudo mkdir -p /etc/systemd/system/ollama.service.d'
        script << "  cat <<'OVERRIDE' | sudo tee /etc/systemd/system/ollama.service.d/override.conf >/dev/null"
        script << '[Service]'
        script << "Environment=\"OLLAMA_HOST=0.0.0.0:#{@config.ollama_port}\""
        script << 'OVERRIDE'
        script << '  sudo systemctl daemon-reload'
        script << '  sudo systemctl restart ollama || true'
        script << 'fi'
      end

      script << 'echo bootstrap-ok'
      script.join("\n")
    end

    def desired_ollama_models
      normalized_model_list(@config.ollama_pull_models)
    end

    def normalized_model_list(models)
      Array(models).each_with_object([]) do |model, ordered|
        normalized = model.to_s.strip
        next if normalized.empty? || ordered.include?(normalized)

        ordered << normalized
      end
    end

    def model_list_signature(models)
      normalized_model_list(models).sort
    end

    def ollama_setup_script
      models_dir = @config.ollama_models_dir
      listen_host = @config.ollama_listen_host
      model_pulls = desired_ollama_models

      script = []
      script << 'set -euo pipefail'
      script << 'sudo pkill -f unattended-upgrade >/dev/null 2>&1 || true'
      script << "if ! command -v ollama >/dev/null 2>&1; then curl -fsSL https://ollama.ai/install.sh | sh; fi"
      if models_dir.start_with?('/ephemeral')
        script << "mountpoint -q /ephemeral || { echo 'Expected /ephemeral mount is missing'; exit 1; }"
      end
      script << "sudo mkdir -p #{Shellwords.escape(models_dir)}"
      script << "sudo chown -R ollama:ollama #{Shellwords.escape(File.dirname(models_dir))}"
      script << 'sudo mkdir -p /etc/systemd/system/ollama.service.d'
      script << "cat <<'OVERRIDE' | sudo tee /etc/systemd/system/ollama.service.d/override.conf >/dev/null"
      script << '[Service]'
      script << "Environment=\"OLLAMA_MODELS=#{models_dir}\""
      script << "Environment=\"OLLAMA_GPU_OVERHEAD=#{@config.ollama_gpu_overhead_mb}\""
      script << "Environment=\"OLLAMA_NUM_PARALLEL=#{@config.ollama_num_parallel}\""
      script << "Environment=\"OLLAMA_HOST=#{listen_host}\""
      script << 'OVERRIDE'
      script << 'sudo systemctl daemon-reload'
      script << 'sudo systemctl enable --now ollama'
      script << 'sudo systemctl restart ollama'
      script << 'sleep 3'
      script << 'systemctl is-active --quiet ollama'
      model_pulls.each do |model|
        script << "ollama pull #{Shellwords.escape(model)}"
      end
      script << "echo ollama-models-dir=#{models_dir}"
      script << 'echo ollama-ok'
      script.join("\n")
    end

    def integer_or_nil(value)
      value.nil? ? nil : Integer(value)
    end

    def print_local_wireguard_summary(expected_ip)
      return unless @config.local_client_checks_enabled?

      wg_status = @local_wireguard.status
      endpoint = wg_status['endpoint']
      info "Local WireGuard #{@config.local_interface_name}: #{wg_status['service_state']}"
      if endpoint
        info "Local WireGuard endpoint: #{endpoint}"
        if expected_ip
          host, = endpoint.split(':', 2)
          if host == expected_ip
            info 'Local WireGuard endpoint matches the managed VM IP.'
          else
            warn "Local WireGuard endpoint points to #{host}, expected #{expected_ip}."
          end
        end
      else
        warn "Unable to read #{@config.local_wg_config_path} for local WireGuard endpoint validation."
      end
    end

    def info(message)
      @out.puts(message)
    end

    def warn(message)
      @out.puts("WARN: #{message}")
    end
  end

  class CLI
    def initialize(argv)
      @argv = argv.dup
    end

    def run
      global = {
        config_path: File.join(__dir__, 'hyperstack-vm.toml')
      }

      global_parser = OptionParser.new do |opts|
        opts.banner = 'Usage: ruby hyperstack_vm.rb [--config path] <create|delete|status> [options]'
        opts.on('--config PATH', "Path to TOML config (default: #{global[:config_path]})") do |value|
          global[:config_path] = value
        end
        opts.on('-h', '--help', 'Show help') do
          puts opts
          puts
          puts 'Commands:'
          puts '  create [--replace] [--dry-run]'
          puts '  delete [--vm-id ID] [--dry-run]'
          puts '  status'
          exit 0
        end
      end
      global_parser.order!(@argv)

      command = @argv.shift
      raise Error, 'Missing command. Use create, delete, or status.' if command.nil?

      config = Config.load(global[:config_path])
      state_store = StateStore.new(config.state_file)
      client = HyperstackClient.new(base_url: config.api_base_url, api_key: config.api_key)
      local_wireguard = LocalWireGuard.new(
        interface_name: config.local_interface_name,
        config_path: config.local_wg_config_path
      )
      manager = Manager.new(
        config: config,
        client: client,
        state_store: state_store,
        local_wireguard: local_wireguard
      )

      case command
      when 'create'
        replace = false
        dry_run = false
        parser = OptionParser.new do |opts|
          opts.on('--replace', 'Delete the tracked VM before creating a new one') { replace = true }
          opts.on('--dry-run', 'Resolve config and print the create plan without creating a VM') { dry_run = true }
        end
        parser.parse!(@argv)
        manager.create(replace: replace, dry_run: dry_run)
      when 'delete'
        vm_id = nil
        dry_run = false
        parser = OptionParser.new do |opts|
          opts.on('--vm-id ID', Integer, 'Delete a VM by ID instead of using the local state file') do |value|
            vm_id = value
          end
          opts.on('--dry-run', 'Show which VM would be deleted without deleting it') { dry_run = true }
        end
        parser.parse!(@argv)
        manager.delete(vm_id: vm_id, dry_run: dry_run)
      when 'status'
        manager.status
      else
        raise Error, "Unknown command #{command.inspect}. Use create, delete, or status."
      end
    end
  end
end

begin
  HyperstackVM::CLI.new(ARGV).run
rescue HyperstackVM::Error => e
  warn "ERROR: #{e.message}"
  exit 1
end
