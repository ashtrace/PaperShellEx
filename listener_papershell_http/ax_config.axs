/// PaperShell HTTP listener

function ListenerUI(mode_create)
{
    // Host selector
    let labelHost = form.create_label("Host & port (Bind):");
    let comboHostBind = form.create_combo();
    comboHostBind.setEnabled(mode_create)
    comboHostBind.clear();
    let addrs = ax.interfaces();
    for (let item of addrs) { comboHostBind.addItem(item); }

    // Port selector
    let spinPortBind = form.create_spin();
    spinPortBind.setRange(1, 65535);
    spinPortBind.setValue(8080);
    spinPortBind.setEnabled(mode_create)

    // Callback selector
    let labelCallback = form.create_label("Callback address:");
    let textCallback = form.create_textline();
    textCallback.setPlaceholder("192.168.1.1:8080");

    // Encryption Key    
    let labelEncryptKey = form.create_label("Encryption key:");
    let textlineEncryptKey = form.create_textline(ax.random_string(32, "hex"));
    textlineEncryptKey.setEnabled(mode_create)
    let buttonEncryptKey = form.create_button("Generate");
    buttonEncryptKey.setEnabled(mode_create)

    // SSL Certificate
    let certSelector = form.create_selector_file();
    certSelector.setPlaceholder("SSL Certificate (optional, auto-generate if empty)");
    let keySelector = form.create_selector_file();
    keySelector.setPlaceholder("SSL Key (optional, auto-generate if empty)");
    let layout_group = form.create_vlayout();
    layout_group.addWidget(certSelector);
    layout_group.addWidget(keySelector);
    let panel_group = form.create_panel();
    panel_group.setLayout(layout_group);
    let ssl_group = form.create_groupbox("Use SSL (HTTPS)", true)
    ssl_group.setPanel(panel_group);
    ssl_group.setChecked(false);

    form.connect(buttonEncryptKey, "clicked", function() { textlineEncryptKey.setText( ax.random_string(32, "hex") ); });

    let layout = form.create_gridlayout();
    let spacer1 = form.create_vspacer();
    let spacer2 = form.create_vspacer();

    layout.addWidget(spacer1,               0, 0, 1, 2);
    layout.addWidget(labelHost,             1, 0, 1, 2);
    layout.addWidget(comboHostBind,         2, 0, 1, 1);
    layout.addWidget(spinPortBind,          2, 1, 1, 1);
    layout.addWidget(labelCallback,         3, 0, 1, 2);
    layout.addWidget(textCallback,          4, 0, 1, 2);
    layout.addWidget(labelEncryptKey,       5, 0, 1, 1);
    layout.addWidget(textlineEncryptKey,    6, 0, 1, 1);
    layout.addWidget(buttonEncryptKey,      6, 1, 1, 1);
    layout.addWidget(ssl_group,             7, 0, 1, 2);
    layout.addWidget(spacer2,               8, 0, 1, 2);

    let container = form.create_container();
    container.put("host_bind", comboHostBind);
    container.put("port_bind", spinPortBind);
    container.put("callback_address", textCallback);
    container.put("encrypt_key", textlineEncryptKey);
    container.put("ssl", ssl_group)
    container.put("ssl_cert", certSelector);
    container.put("ssl_key", keySelector);


    let panel = form.create_panel();
    panel.setLayout(layout);

    return {
        ui_panel: panel,
        ui_container: container
    }
}
