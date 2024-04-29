#!/bin/bash

# Define the path to the buildfiles directory
root_dir="/home/emikar/repos/thesis/report"
build_dir="$root_dir/buildfiles"

# Define the file to be compiled
default_file="thesis"
# default_file="prestudy"
file=${1:-$default_file}

# Create the buildfiles directory if it doesn't already exist
cd "$root_dir"
mkdir -p "$build_dir"
# Copy the necessary files and directories to the build directory
cp -r img "$build_dir/" # Copy the figures directory recursively
cp -r plt "$build_dir/" # Copy the figures directory recursively
cp "$file.tex" "$build_dir/"
cp references.bib "$build_dir/"
cp kththesis.cls "$build_dir/"

cd "$build_dir"
pdflatex -interaction=nonstopmode "$file.tex"
makeglossaries "$file"
biber "$file"
pdflatex -interaction=nonstopmode "$file.tex"
pdflatex -interaction=nonstopmode "$file.tex"
mv "$file.pdf" "../"

# Go back to the original directory
cd -