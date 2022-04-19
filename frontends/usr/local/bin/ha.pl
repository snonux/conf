#!/usr/bin/env perl

use strict;
use warnings;

use HTTP::Tiny;
use IO::Socket::INET;
use Sys::Hostname;
use JSON::PP;
use File::Copy;
use Data::Dumper;

use constant {
    STATUS_FILE => '/var/run/ha.status',
    TMP_STATUS_FILE => '/tmp/ha.status',
    PARTICIPANTS => qw(blowfish.buetow.org twofish.buetow.org),
    HA_STATUS_PORT => 4242,
    MAX_STATUS_AGE => 60,
}

sub update_ha_status {
    my @status = @_;
    my $json = JSON::PP->new->ascii;

    open my $fd, '>', TMP_STATUS_FILE or die $!;
    print $fd $json->encode($_), "\n" for @status;
    close $fd;

    copy TMP_STATUS_FILE, STATUS_FILE or die $!;
    unlink TMP_STATUS_FILE;
}

sub fetch_remote_ha_status {
    my $peer = shift;
    my $socket = new IO::Socket::INET (
        PeerHost => $peer,
        PeerPort => HA_STATUS_PORT,
        Proto => 'tcp',
    );
    return undef unless $socket;

    my $response = '';
    $socket->recv($response, 4096);
    $socket->close();
    return split /\n/, $response;
}

sub check_http_status {
    my $peer = shift;
    my $response = HTTP::Tiny->new( max_redirect => 0)->get('http://' . $peer);
    my $valid_response = $response->{'status'} >= 200 &&
                         $response->{'status'} < 400;
    return {
        endpoint => 'http://' . $peer,
        peer => $peer,
        checked_from => hostname,
        status => $valid_response ? 'OK' : 'ERROR',
        message => $valid_response ? 'All fine' : 'Got unexpeced response',
        epoch => time,
    }
}

sub check_gemini_status {
    my $peer = shift;
    my $socket = new IO::Socket::INET (
        PeerHost => $peer,
        PeerPort => 1965,
        Proto => 'tcp',
    );

    my $status = {
        endpoint => 'gemini://' . $peer,
        peer => $peer,
        checked_from => hostname,
        status => $socket ? 'OK' : 'ERROR',
        message => $socket ? 'All fine' : $!,
        epoch => time,
    };

    $socket->close() if $socket;
    return $status;
}

sub check_status {
    my $peer = shift;
    my @service_status;

    push @service_status, check_http_status $peer;
    push @service_status, check_gemini_status $peer;

    update_ha_status @service_status;
    return @service_status;
}

sub scores {
    my %scores;

    for my $status (@_) {
        next if time - $status->{epoch} > MAX_STATUS_AGE;
        if ($status->{status} eq 'OK') {
            $scores{$status->{peer}}++;
        } else {
            $scores{$status->{peer}} |= 0;
        }
    }

    return
        map { [$_, $scores{$_}] }
        sort { $scores{$b} <=> $scores{$a} }
        keys %scores;
}

sub main {
    my $json = JSON::PP->new->ascii;
    my $hostname = hostname;
    my @all;

    for my $partner (grep { $_ ne $hostname } PARTICIPANTS) {
        for (check_status $partner) {
            print $json->encode($_), "\n";
            push @all, $_;
        }
        for (fetch_remote_ha_status $partner) {
            next if not defined or /^\s*#/;
            print "$_\n";
            push @all, $json->decode($_);
        }
    }

    print Dumper scores @all;
}

main;
