#!/usr/bin/env python3
"""
Script to copy and transform generated CRDs to Helm chart templates.
"""

import os
import re

def update_helm_crds():
    helm_dir = "../helm/kontroler/templates/crds"
    crd_files = {
        "kontroler.greedykomodo_dags.yaml": "dags.yaml",
        "kontroler.greedykomodo_dagruns.yaml": "dagrun.yaml",
        "kontroler.greedykomodo_dagtasks.yaml": "dagtasks.yaml"
    }
    
    # Create helm crds directory if it doesn't exist
    os.makedirs(helm_dir, exist_ok=True)
    
    for src_file, dest_file in crd_files.items():
        src_path = f"config/crd/bases/{src_file}"
        dest_path = f"{helm_dir}/{dest_file}"
        
        if os.path.exists(src_path):
            with open(src_path, "r") as f:
                content = f.read()
            
            # Remove leading ---
            content = re.sub(r"^---\n", "", content)
            
            # Add Helm templating to the annotations
            content = re.sub(
                r"\nmetadata:\n  annotations:\n    controller-gen\.kubebuilder\.io/version: v[0-9.]+\n  name:",
                r"\nmetadata:\n  annotations:\n    controller-gen.kubebuilder.io/version: v0.13.0\n    {{ if .Values.crds.retain }}\n    helm.sh/resource-policy: keep\n    {{ end }}\n  name:",
                content
            )
            
            # Wrap with Helm conditional
            helm_content = f"{{{{ if .Values.crds.install }}}}\n{content}{{{{ end }}}}"
            
            with open(dest_path, "w") as f:
                f.write(helm_content)
            
            print(f"Updated {dest_file}")
        else:
            print(f"Warning: {src_path} not found")

if __name__ == "__main__":
    update_helm_crds()