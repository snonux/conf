#!/usr/bin/perl -T

use v5.36;
use strict;
use warnings;

use Acme::Cow;

our @habits = (
  '1h+ deep work',
  'Breathing exercise',
  'Do nothing',
  'Do pistol squads',
  'Do Push-ups',
  'Do Shoulder stand',
  'Do single-arm push-ups',
  'Do Yoga Bridge',
  'Do Yoga Cro',
  'Do Yoga Downward Facing Dog',
  'Do Yoga Half Moon',
  'Do Yoga Warrior 3',
  'Do Yoga Wheel',
  'Drink tea (or an infusion)',
  'Drink water',
  'Eat 99% dark chocolate',
  'Enjoy my current task',
  'Focus on things I have under control',
  'Got enough vitamins?',
  'Have a positive attitute - Be solution oriented',
  'Watch bulgarian TV or YouTube',
  'Have a walk without headphones plugged in',
  'Help someone',
  'It\'s family time',
  'Learn/try out a new Linux/Unix command',
  'Learn vocs with Anki',
  'Limit the use of social media',
  'Listen to a random 101 chapter today',
  'Listen to music',
  'Meditate',
  'Only use my phone and computers intentionally',
  'Play with the cat',
  'Process my last (book) notes',
  'Read my Gemini subscriptions',
  "Read today's chapter of the daily stoic",
  'Revisit my core values',
  'Take it easy - nobody is dying!',
  'Think about the purpose of what I am doing',
  'Tonight, disconnect from work completely',
  'Write down 10 ideas (weird and non-weird)',
  'Write down 3 things I am grateful for',
);

my $habit = $habits[rand @habits];
my $cow = Acme::Cow->new;
$cow->text($habit);
int(rand 2) ? $cow->say : $cow->think;
$cow->print;
