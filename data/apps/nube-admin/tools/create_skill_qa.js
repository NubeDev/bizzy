// create_skill_qa.js — guided skill builder.
//
// Walks the user through creating a new app with:
//   - app.yaml
//   - A prompt template
//   - A QA JS tool that asks questions and renders the prompt
//
// The generated files are written to the apps/ directory via the
// nube-server REST API (POST /apps, POST /apps/:id/tools, etc.).
// This tool returns the API calls as structured output so the server
// can execute them.

function handle(params) {
  if (params._answers !== undefined) {
    return chatMode(params._answers);
  }
  if (!params._submit) {
    return formDefinition();
  }
  return formSubmit(params);
}

// ---- Chat mode ----

function chatMode(answers) {
  if (!answers.skill_name) {
    return {
      type: "question",
      field: "skill_name",
      label: "What should this skill be called?",
      input: "text",
      required: true,
      min_length: 3,
      placeholder: "e.g. sales-proposal, site-report, onboarding-checklist"
    };
  }

  if (!answers.description) {
    return {
      type: "question",
      field: "description",
      label: "What does \"" + answers.skill_name + "\" do? Describe it in one sentence.",
      input: "text",
      required: true,
      placeholder: "e.g. Generates a sales proposal for a prospective customer"
    };
  }

  if (!answers.questions_raw) {
    return {
      type: "question",
      field: "questions_raw",
      label: "What questions should it ask the user? List them, one per line.",
      input: "textarea",
      required: true,
      min_length: 5,
      placeholder: "Company name\nProduct interest\nDeal size\nTimeline\nCompetitor info"
    };
  }

  if (!answers.output_description) {
    return {
      type: "question",
      field: "output_description",
      label: "What should the output look like? Describe the format and content.",
      input: "textarea",
      required: true,
      placeholder: "e.g. A 1-page proposal with company overview, solution fit, pricing table, and next steps"
    };
  }

  if (!answers.tags) {
    return {
      type: "question",
      field: "tags",
      label: "Any tags to categorize this skill?",
      input: "text",
      required: false,
      placeholder: "e.g. sales, proposals (comma-separated, optional)"
    };
  }

  if (!answers.confirmed) {
    return {
      type: "question",
      field: "confirmed",
      label: "Ready to create \"" + answers.skill_name + "\"?\n\n" +
             "This will create:\n" +
             "  - apps/" + answers.skill_name + "/app.yaml\n" +
             "  - apps/" + answers.skill_name + "/prompts/" + safeName(answers.skill_name) + ".md\n" +
             "  - apps/" + answers.skill_name + "/tools/" + safeName(answers.skill_name) + "_qa.js\n" +
             "  - apps/" + answers.skill_name + "/tools/" + safeName(answers.skill_name) + "_qa.json",
      input: "select",
      required: true,
      options: [
        { value: "yes", label: "Yes, create it" },
        { value: "no", label: "No, cancel" }
      ]
    };
  }

  if (answers.confirmed !== "yes") {
    return { type: "prompt", prompt: "Skill creation cancelled." };
  }

  return generateSkill(answers);
}

// ---- Form mode ----

function formDefinition() {
  return {
    type: "qa",
    title: "Skill Builder",
    description: "Create a new skill with a guided QA flow.",
    fields: [
      {
        name: "skill_name", label: "Skill name",
        type: "text", required: true, min_length: 3,
        placeholder: "e.g. sales-proposal"
      },
      {
        name: "description", label: "Description",
        type: "text", required: true,
        placeholder: "What does this skill do?"
      },
      {
        name: "questions_raw", label: "Questions to ask (one per line)",
        type: "textarea", required: true,
        placeholder: "Company name\nProduct interest\nDeal size"
      },
      {
        name: "output_description", label: "Output description",
        type: "textarea", required: true,
        placeholder: "What should the output look like?"
      },
      {
        name: "tags", label: "Tags (comma-separated)",
        type: "text", required: false
      }
    ]
  };
}

function formSubmit(params) {
  var errors = [];
  if (!params.skill_name || params.skill_name.length < 3) {
    errors.push({ field: "skill_name", message: "Skill name required (min 3 chars)" });
  }
  if (!params.description) {
    errors.push({ field: "description", message: "Description required" });
  }
  if (!params.questions_raw) {
    errors.push({ field: "questions_raw", message: "At least one question required" });
  }
  if (!params.output_description) {
    errors.push({ field: "output_description", message: "Output description required" });
  }
  if (errors.length > 0) {
    return { type: "validation_error", errors: errors };
  }
  return generateSkill(params);
}

// ---- Generate the skill files ----

function generateSkill(answers) {
  var name = answers.skill_name.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "");
  var safe = safeName(name);
  var questions = parseQuestions(answers.questions_raw);
  var tags = (answers.tags || "").split(",").map(function(t) { return t.trim(); }).filter(function(t) { return t; });

  // Generate the prompt template.
  var promptBody = answers.description + "\n\n";
  for (var i = 0; i < questions.length; i++) {
    var q = questions[i];
    promptBody += q.label + ": {{" + q.field + "}}\n";
  }
  promptBody += "\n" + answers.output_description + "\n";

  // Generate the QA JS tool.
  var jsCode = generateJS(safe, questions, answers.description);

  // Generate the tool manifest.
  var toolParams = { "_answers": { type: "object", required: false, description: "Accumulated answers" } };
  for (var j = 0; j < questions.length; j++) {
    toolParams[questions[j].field] = {
      type: "string",
      required: false,
      description: questions[j].label
    };
  }

  return {
    type: "prompt",
    title: "Create skill: " + name,
    prompt: "I've prepared the skill \"" + name + "\". Please create it using the nube-server API.\n\n" +
            "Use these API calls in order:\n\n" +
            "1. POST /apps — create the app\n" +
            "2. POST /apps/" + name + "/prompts — create the prompt template\n" +
            "3. POST /apps/" + name + "/tools — create the QA tool\n\n" +
            "Then install it for the user.",

    // Structured output: the server or Claude can use these to make the API calls.
    create_app: {
      name: name,
      description: answers.description,
      author: "NubeIO",
      tags: tags,
      version: "1.0.0"
    },
    create_prompt: {
      name: safe,
      description: answers.description,
      arguments: questions.map(function(q) {
        return { name: q.field, description: q.label, required: q.required };
      }),
      body: promptBody
    },
    create_tool: {
      name: safe + "_qa",
      description: answers.description + " (guided builder)",
      mode: "qa",
      toolClass: "read-only",
      params: toolParams,
      script: jsCode
    }
  };
}

// ---- Generate JS code for the QA tool ----

function generateJS(safe, questions, description) {
  var lines = [];
  lines.push("// " + safe + "_qa.js — auto-generated QA flow.");
  lines.push("// " + description);
  lines.push("");
  lines.push("function handle(params) {");
  lines.push("  if (params._answers !== undefined) {");
  lines.push("    return chatMode(params._answers);");
  lines.push("  }");
  lines.push("  if (!params._submit) {");
  lines.push("    return { type: 'qa', title: '" + escape(description) + "', fields: getFields() };");
  lines.push("  }");
  lines.push("  return submitForm(params);");
  lines.push("}");
  lines.push("");

  // Chat mode.
  lines.push("function chatMode(answers) {");
  for (var i = 0; i < questions.length; i++) {
    var q = questions[i];
    lines.push("  if (!answers." + q.field + ") {");
    lines.push("    return {");
    lines.push("      type: 'question',");
    lines.push("      field: '" + q.field + "',");
    lines.push("      label: '" + escape(q.label) + "',");
    lines.push("      input: '" + q.input + "',");
    lines.push("      required: " + q.required);
    lines.push("    };");
    lines.push("  }");
  }
  lines.push("  return renderPrompt(answers);");
  lines.push("}");
  lines.push("");

  // Form fields.
  lines.push("function getFields() {");
  lines.push("  return [");
  for (var j = 0; j < questions.length; j++) {
    var f = questions[j];
    lines.push("    { name: '" + f.field + "', label: '" + escape(f.label) + "', type: '" + f.input + "', required: " + f.required + " },");
  }
  lines.push("  ];");
  lines.push("}");
  lines.push("");

  // Submit form.
  lines.push("function submitForm(params) {");
  lines.push("  var errors = [];");
  for (var k = 0; k < questions.length; k++) {
    if (questions[k].required) {
      lines.push("  if (!params." + questions[k].field + ") errors.push({ field: '" + questions[k].field + "', message: '" + escape(questions[k].label) + " is required' });");
    }
  }
  lines.push("  if (errors.length > 0) return { type: 'validation_error', errors: errors };");
  lines.push("  return renderPrompt(params);");
  lines.push("}");
  lines.push("");

  // Render prompt.
  lines.push("function renderPrompt(answers) {");
  lines.push("  var template = files.read('prompts/" + safe.replace(/_/g, "-") + ".md');");
  lines.push("  var parts = template.split('---');");
  lines.push("  var body = parts.slice(2).join('---').trim();");
  for (var m = 0; m < questions.length; m++) {
    lines.push("  body = body.replace(/\\{\\{" + questions[m].field + "\\}\\}/g, answers." + questions[m].field + " || '');");
  }
  lines.push("  return { type: 'prompt', prompt: body };");
  lines.push("}");

  return lines.join("\n");
}

// ---- Helpers ----

function parseQuestions(raw) {
  var lines = raw.split("\n");
  var questions = [];
  for (var i = 0; i < lines.length; i++) {
    var line = lines[i].trim();
    if (!line) continue;

    // Derive a field name from the question text.
    var field = line.toLowerCase()
      .replace(/[^a-z0-9\s]/g, "")
      .replace(/\s+/g, "_")
      .substring(0, 30);

    // Detect if it's a long-form question.
    var input = "text";
    var lower = line.toLowerCase();
    if (lower.indexOf("describe") >= 0 || lower.indexOf("detail") >= 0 ||
        lower.indexOf("explain") >= 0 || lower.indexOf("notes") >= 0) {
      input = "textarea";
    }

    questions.push({
      field: field,
      label: line,
      input: input,
      required: i < 2 // first two questions are required
    });
  }
  return questions;
}

function safeName(name) {
  return name.replace(/-/g, "_");
}

function escape(s) {
  return s.replace(/'/g, "\\'");
}
