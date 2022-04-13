#!/usr/bin/env perl

use strict;
use warnings;

use Sys::Hostname;
use Getopt::Long;

use constant STATUS_FILE => '/var/run/failover.status';
use constant PARTNERS => qw(blowfish.buetow.org twofish.buetow.org);

sub slurp {
  my $file_path = shift;
  open my $fd, $file_path or die $!;
  my $data = <$fd>;
  close $fd;
  return $data;
}

sub score {
  for (PARTNERS) {
    next if $_ eq hostname; # Ignore self
    print "Trying $_\n";
  }
}

sub inetd_response {
  my $hostname = hostname;
  print "OK: All is fine on $hostname itself!\n";
  print slurp STATUS_FILE if -f STATUS_FILE;
}

sub main {
  my ($inetd, $score) = (0, 0);
  GetOptions('inetd' => \$inetd,
             'score' => \$score)
    or die "Error in command line arguments!\n";

  score if $score;
  inetd_response if $inetd;
}

main;
