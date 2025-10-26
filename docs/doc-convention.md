# Documentation Conventions

This document outlines the conventions for writing and maintaining documentation in the `edge-cd` repository.

## README.md Files

Every package and subfolder in this repository should have a `README.md` file that briefly explains its purpose. This helps developers quickly understand the structure of the project and the role of each component.

### Table of Contents

The main `README.md` file should have a Table of Contents (TOC) at the top to help users navigate the document. The TOC should be kept up-to-date with the latest changes to the document.

### Cross-Referencing

To make the documentation easy to navigate, `README.md` files should include cross-references to other relevant documents. These references should be placed in a "See Also" section at the bottom of the file.

For example, the main `README.md` should include links to the `README.md` files for the command-line tools in the `./cmd` directory, and vice versa.

## GoDoc Comments

All public functions, methods, and types should have comprehensive GoDoc comments that explain their purpose, parameters, and return values. This is crucial for making the code easy to understand and maintain.
