#!/usr/bin/perl -T

use v5.36;
use strict;
use warnings;

use Acme::Cow;

our @habits = (
  'Do Push-ups',
  'Do Yoga Wheel',
  'Do Yoga Cro',
  'Do Yoga Downward Facing Dog',
  'Do Yoga Warrior 3',
  'Do Yoga Half Moon',
  'Do Shoulder stand',
  'Meditate',
  'Listen to music',
  'Do nothing',
  'Breathing exercise',
  'Play with the cat',
  'Drink water',
  'Think about the purpose of what I am doing',
  'Write down 3 things I am grateful for',
  'Take it easy - nobody is dying!',
  'Enjoy my current task',
  'It\'s family time',
  'Help someone',
  'Tonight, disconnect from work completely',
  'Limit the use of social media',
  'Only use my phone and computers intentionally',
  'Learn vocs with Anki',
  'Learn/try out a new Linux/Unix command',
);

my $habit = $habits[rand @habits];
my $cow = Acme::Cow->new;
$cow->text($habit);
int(rand 2) ? $cow->say : $cow->think;
$cow->print;
